# File Matching

The operator searches the file system for files that meet the following requirements:
1. The file's path matches one or more patterns specified in the `include` setting.
2. The file's path does not match any pattern specified in the `exclude` setting.

The set of files that satisfy these requirements are known in this document as "matched files". 
The effective search space (`include - exclude`) is referred to colloquially as the operator's "matching pattern".

# Fingerprints

Files are identified and tracked using fingerprints. A fingerprint is the first `N` bytes of the file,
with the default for `N` being `1000`. 

### Fingerprint Growth

When a file is smaller than `N` bytes, the fingerprint is the entire contents of the file. A fingerprint that is
less than `N` bytes will be compared to other fingerprints using a prefix check. As the file grows, its fingerprint
will be updated, until it reaches the full size of `N`.

### Deduplication of Files

Multiple files with the same fingerprint are handled as if they are the same file. 

Most commonly, this circumstance is observed during file rotation that depends on a copy/truncate strategy.
After copying the file, but before truncating the original, two files with the same content briefly exist.
If the `file_input` operator happens to observe both files at the same time, it will detect a duplicate fingerprint
and ingest only one of the files.

If logs are replicated to multiple files, or if log files are copied manually, it is not understood to be of any
significant value to ingest the duplicates. As a result, fingerprints are not designed to differentiate between these
files, and double ingestion of the same content is not supported automatically.

In some rare circumstances, a logger may print a very verbose preamble to each log file. When this occurs,
fingerprinting may fail to differentiate files from one another. This can be overcome by customizing the size
of the fingerprint using the `fingerprint_size` setting.

### Log line ordering across file rotations

In general, we offer no guarantees as to the relative ordering of log lines originating from different files.
For the common use case of files being rotated outside the watched pattern, we make a best-effort attempt at reading
the rotated file to the end before reading the new file. This guarantees log line ordering across rotations,
assuming the following conditions are met:

* rotated file names don't match the watched pattern
* rotated files aren't written to after the rotation

A minor reordering of log lines often doesn't matter, but it can when using the recombine operator later in the
pipeline, for example.

# Readers

Readers are a convenience struct, which exist for the purpose of managing files and their associated metadata. 

### Contents

A Reader contains the following:
- File handle (may be open or closed)
- File fingerprint
- File offset (aka checkpoint)
- File path
- Decoder (dedicated instance to avoid concurrency issues)

### Functionality

As implied by the name, Readers are responsible for consuming data as it is written to a file.

Before a Reader begins consuming, it will seek the file's last known offset. If no offset is known for the file, then
the Reader will seek either the beginning or end of the file, according to the `start_at` setting. It will then begin
reading from there.

While a file is shorter than the length of a fingerprint, its Reader will continuously append to the fingerprint,
as it consumes newly written data.

A Reader consumes a file using a `bufio.Scanner`, with the Scanner's buffer size defined by the `max_log_size` setting,
and the Scanner's split func defined by the `multiline` setting. 

As each log is read from the file, it is decoded according to the `encoding` function, and then emitted from 
the operator. 

The Reader's offset is updated accordingly whenever a log is emitted.


### Persistence

Readers are always instantiated with an open file handle. Eventually, the file handle is closed, but the Reader is
not immediately discarded. Rather, it is maintained for a fixed number of "poll cycles" (see Polling section below)
as a reference to the file's metadata, which may be useful for detecting files that have been moved or copied,
and for recalling metadata such as the file's previous path.

Readers are maintained for a fixed period of time, and then discarded.

When the `file_input` operator makes use of a persistence mechanism to save and recall its state, it is simply
Setting and Getting a slice of Readers. These Readers contain all the information necessary to pick up exactly
where the operator left off.


# Polling

The file system is polled on a regular interval, defined by the `poll_interval` setting. 

Each poll cycle runs through a series of steps which are presented below.

### Detailed Poll Cycle

1. Dequeuing
    1. If any matches are queued from the previous cycle, an appropriate number are dequeued, and processed the same
       as would a newly matched set of files.
2. Aging
    1. If no queued files were left over from the previous cycle, then all previously matched files have been consumed,
       and we are ready to query the file system again. Prior to doing so, we will increment the "generation" of all
       historical Readers. Eventually, these Readers will be discarded based on their age. Until that point, they may
       be useful references.
3. Matching
    1. The file system is searched for files with a path that matches the `include` setting.
    2. Files that match the `exclude` setting are discarded.
    3. As a special case, on the first poll cycle, a warning is printed if no files are matched.
       Execution continues regardless.
4. Queueing
    1. If the number of matched files is less than or equal to the maximum degree of concurrency, as defined
       by the `max_concurrent_files` setting, then no queueing occurs.
    2. Else, queueing occurs, which means the following:
        - Matched files are split into two sets, such that the first is small enough to respect `max_concurrent_files`,
          and the second contains the remaining files (called the queue).
        - The current poll interval will begin processing the first set of files, just as if they were the
          only ones found during the matching phase.
        - Subsequent poll cycles will pull matches off of the queue, until the queue is empty.
        - The `max_concurrent_files` setting is respected at all times.
5. Opening
    1. Each of the matched files is opened. Note:
        - A small amount of time has passed since the file was matched.
        - It is possible that it has been moved or deleted by this point.
        - Only a minimum set of operations should occur between file matching and opening.
        - If an error occurs while opening, it is logged.
6. Fingerprinting
    1. The first `N` bytes of each file are read. (See fingerprinting section above.)
7. Exclusion
    1. Empty files are closed immediately and discarded. (There is nothing to read.)
    2. Fingerprints found in this batch are cross referenced against each other to detect duplicates. Duplicate
       files are closed immediately and discarded.
        - In the vast majority of cases, this occurs during file rotation that uses the copy/truncate method.
          (See fingerprinting section above.)
8. Reader Creation
    1. Each file handle is wrapped into a `Reader` along with some metadata. (See Reader section above)
        - During the creation of a `Reader`, the file's fingerprint is cross referenced with previously
          known fingerprints.
        - If a file's fingerprint matches one that has recently been seen, then metadata is copied over from the
          previous iteration of the Reader. Most importantly, the offset is accurately maintained in this way.
        - If a file's fingerprint does not match any recently seen files, then its offset is initialized
          according to the `start_at` setting.
9. Detection of Lost Files
    1. Fingerprints are used to cross reference the matched files from this poll cycle against the matched
       file from the previous poll cycle. Files that were matched in the previous cycle but were not matched
       in this cycle are referred to as "lost files".
    2. File become "lost" for several reasons:
        - The file may have been deleted, typically due to rotation limits or ttl-based pruning.
        - The file may have been rotated to another location.
            - If the file was moved, the open file handle from the previous poll cycle may be useful.
10. Consumption
    1. Lost files are consumed. In some cases, such as deletion, this operation will fail. However, if a file
       was moved, we may be able to consume the remainder of its content. 
        - We do not expect to match this file again, so the best we can do is finish consuming their current contents.
        - We can reasonably expect in most cases that these files are no longer being written to.
    2. Matched files (from this poll cycle) are consumed.
        - These file handles will be left open until the next poll cycle, when they will be used to detect and
          potentially consume lost files.
        - Typically, we can expect to find most of these files again. However, these files are consumed greedily
          in case we do not see them again.
    3. All open files are consumed concurrently. This includes both the lost files from the previous cycle, and the
       matched files from this cycle.
11. Closing
    1. All files from the previous poll cycle are closed.
12. Archiving
    1. Readers created in the current poll cycle are added to the historical record.
    2. The same Readers are also retained as a separate slice, for easy access in the next poll cycle.
13. Pruning
    1. The historical record is purged of Readers that have existed for 3 generations.
        - This number is somewhat arbitrary, and should probably be made configurable. However, its exact purpose
          is quite obscure.
14. Persistence
    1. The historical record of readers is synced to whatever persistence mechanism was provided to the operator.
15. End Poll Cycle
    1. At this point, the operator sits idle until the poll timer fires again.



# Additional Details

### Startup Logic

Whenever the operator starts, it:
- Requests the historical record of Readers, as described in steps 12-14 of the poll cycle.
- Starts the polling timer.

### Shutdown Logic

When the operator shuts down, the following occurs:
- If a poll cycle is not currently underway, the operator simply closes any open files.
- Otherwise, the current poll cycle is signaled to stop immediately, which in turn signals all Readers to
  stop immediately.
    - If a Reader is idle or in between log entries, it will return immediately. Otherwise it will return
      after consuming one final log entry.
    - Once all Readers have stopped, the remainder of the poll cycle completes as usual, which includes
      the steps labeled `Closing`, `Archiving`, `Pruning`, and `Persistence`.

The net effect of the shut down routine is that all files are checkpointed in a normal manner
(i.e. not in the middle of a log entry), and all checkpoints are persisted.

### Log rotation

#### Supported cases

A) When a file is moved within the pattern with unread logs on the end, then the original is created again,
   we get the unread logs on the moved as well as any new logs written to the newly created file.

B) When a file is copied within the pattern with unread logs on the end, then the original is truncated,
   we get the unread logs on the copy as well as any new logs written to the truncated file.

C) When a file it rotated out of pattern via move/create, we detect that
   our old handle is still valid and we attempt to read from it.

D) When a file it rotated out of pattern via copy/truncate, we detect that
   our old handle is invalid and we do not attempt to read from it.


#### Rotated files that end up within the matching pattern

In both cases of copy/truncate and move/create, if the rotated files match the pattern
then the old readers that point to the original path will be closed and we will create new
ones which will be pointing to the rotated file but using the existing metadata's offset.
The receiver will continue consuming the rotated paths in any case so there will be
no data loss during the transition.
The original files will have a fresh fingerprint so they will be consumed by a completely
new reader.

#### Rotated files that end up out of the matching pattern

In case of a file has been rotated with move/create, the old handle will be pointing
to the moved file so we can still consume from it even if it's out of the pattern.
In case of the file has been rotated with copy/truncate, the old handle will be pointing
to the original file which is truncated. So we don't have a handle in order to consume any remaining
logs from the moved file. This can cause data loss.

# Known Limitations

### Potential data loss when maximum concurrency must be enforced

The operator may lose a small percentage of logs, if both of the following conditions are true:
1. The number of files being matched exceeds the maximum degree of concurrency allowed
   by the `max_concurrent_files` setting. 
2. Files are being "lost". That is, file rotation is moving files out of the operator's matching pattern,
   such that subsequent polling cycles will not find these files.

When both of these conditions occur, it is impossible for the operator to both:
1. Respect the specified concurrency limitation.
2. Guarantee that if a file is rotated out of the matching pattern, it may still be consumed before being closed.

When this scenario occurs, a design tradeoff must be made. The choice is between:
1. Ensure that `max_concurrent_files` is always respected.
2. Risk losing a small percentage of log entries.

The current design chooses to guarantee the maximum degree of concurrency because failure to do so risks
harming the operator's host system. While the loss of logs is not ideal, it is less likely to harm
the operator's host system, and is therefore considered the more acceptable of the two options.

### Potential data loss when file rotation via copy/truncate rotates backup files out of operator's matching pattern

The operator may lose a small percentage of logs, if both of the following conditions are true:
1. Files are being rotated using the copy/truncate strategy.
2. Files are being "lost". That is, file rotation is moving files out of the operator's matching pattern,
   such that subsequent polling cycles will not find these files.

When both of these conditions occur, it is possible that a file is written to (then copied elsewhere) and
then truncated before the operator has a chance to consume the new data.

### Potential failure to consume files when file rotation via move/create is used on Windows

On Windows, rotation of files using the Move/Create strategy may cause errors and loss of data,
because Golang does not currently support the Windows mechanism for `FILE_SHARE_DELETE`.
