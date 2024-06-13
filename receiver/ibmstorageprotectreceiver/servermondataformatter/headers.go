package servermondataformatter

func GetColumns(query string) []string {
	switch query {
	case "QUERY PROCESS":
		return []string{"Process Number", "Process Description", "Job Id", "Process Status", "Parent Process"}

	case "QUERY DB FORMAT=DETAILED":
		return []string{
			"Database Name", "Total Space of File System(MB)", "Space Used on File System(MB)",
			"Space Used by Database(MB)", "Free Space Available(MB)", "Total Pages", "Usable Pages",
			"Used Pages", "Free Pages", "Buffer Pool Hit Ratio", "Total Buffer Requests", "Sort Overflows",
			"Package Cache Hit Ratio", "Last Database Reorganization", "Full Device Class Name",
			"Number of Database Backup Streams", "Incrementals Since Last Full", "Last Complete Backup Date/Time",
			"Compress Database Backups", "Encrypt Database Backups", "Protect Master Encryption Key",
			"Database Backup Password Set", "Servermon Total Used Space(MB)",
		}

	case "QUERY DBSPACE FORMAT=DETAILED":
		return []string{"Location", "Total Space of File System(MB)", "Used Space on File System", "Free Space(MB)"}

	case "QUERY LOG FORMAT=DETAILED":
		return []string{
			"Active Log Directory", "Total Space(MB)", "Used Space(MB)", "Free Space(MB)",
			"Total Space of File System(MB)", "Used Space on File System(MB)", "Free Space on File System(MB)",
			"Archive Log Directory", "Total Space of File System(MB)", "Used Space on File System(MB)",
			"Archive Log Compressed", "Mirror Log Directory", "Total Space of File System(MB)",
			"Used Space on File System(MB)", "Free Space on File System(MB)", "Archive Failover Log Directory",
			"Total Space of File System(MB)", "Used Space on File System(MB)", "Free Space on File System(MB)",
		}

	case "QUERY DEVCLASS FORMAT=DETAILED":
		return []string{
			"Device Class Name", "Device Access Strategy", "Storage Pool Count", "Device Type",
			"Format", "Est/Max Capacity(MB)", "Mount Limit", "Mount Wait(min)", "Mount Retension(min)",
			"Label Prefix", "Library", "Directory", "Server Name", "Retry Period", "Retry Interval",
			"Shared", "High-level Address", "Minimum Capacity", "WORM", "Drive Encryption",
			"Scaled Capacity", "Primary Allocation(MB)", "Secondary Allocation(MB)", "Compression",
			"Retension", "Protection", "Expiration Date", "Unit", "Logical Block Protection",
			"Connection Name", "Cloud Storage Class", "Last Update by (administrator)", "Last Update Date/Time",
		}

	case "QUERY STGPOOL FORMAT=DETAILED":
		return []string{
			"Storage Pool Name", "Storage Pool Type", "Device Class Name", "Storage Type", "Connection Name",
			"Cloud Storage Class", "Estimated Capacity", "Space Trigger Util", "Pct Util", "Pct Migr",
			"Pct Logical", "High Mig Pct", "Low Mig Pct", "Migration Delay", "Migration Continue",
			"Migration Processes", "Reclamation Processes", "Next Storage Pool", "Reclaim Storage Pool",
			"Maximum Size Threshold", "Access", "Description", "Overflow Location", "Cache Migrated Files?",
			"Collocate?", "Reclamation Threshold", "Offsite Reclamation Limit", "Maximum Scratch Volumes Allowed",
			"Number of Scratch Volumes Used", "Delay Period for Container Reuse", "Migration in Progress?",
			"Amount Migrated (MB)", "Elapsed Migration Time (seconds)", "Reclamation in Progress?",
			"Last Update by (administrator)", "Last Update Date/Time", "Storage Pool Data Format",
			"Copy Storage Pool(s)", "Active Data Pool(s)", "Continue Copy on Error?", "CRC Data",
			"Reclamation Type", "Overwrite Data When Deleted", "Compressed", "Compression Savings",
			"Deduplicate Data?", "Processes For Identifying Duplicates", "Space Used for Protected Data",
			"Total Pending Space", "Deduplication Savings", "Total Space Saved", "Auto-copy Mode",
			"Contains Data Deduplicated by Client?", "Maximum Simultaneous Writes", "Protect Processes",
			"Protect Storage Pool", "Protect Local Storage Pool(s)", "Reclamation Volume Limit",
			"Date of Last Protection to Remote Pool", "Date of Last Protection to Local Pool",
			"Deduplicate Requires Backup?", "Encrypted", "Pct Encrypted", "Cloud Space Allocated (MB)",
			"Cloud Space Utilized (MB)", "Local Estimated Capacity", "Local Pct Util", "Local Pct Logical",
			"Remove Restored Cpy Before End of Life", "Cloud Read Cache", "Cloud Data Locking",
			"Cloud Data Lock Duration (Days)",
		}

	case "QUERY STGPOOLDIRECTORY FORMAT=DETAILED":
		return []string{
			"Storage Pool Name", "Directory", "Access", "Free Space(MB)", "Total Space(MB)", "File System",
			"Absolute Path", "Read Cache Space Used (MB)",
		}

	case "QUERY REPLICATION * STATUS=RUNNING FORMAT=DETAILED":
		return []string{
			"Node Name", "Filespace Name", "FSID", "Start Time", "End Time", "Status", "Process Number",
			"Command", "Storage Rule Name", "Phase", "Process Running Time", "Completion State",
			"Reason for Incompletion", "Backup Last Update Date/Time", "Backup Target Server",
			"Backup Files Needing No Action", "Backup Files To Replicate", "Backup Files Replicated",
			"Backup Files Not Replicated Due to Errors", "Backup Files Not Yet Replicated",
			"Backup Files To Delete", "Backup Files Deleted", "Backup Files Not Deleted Due To Errors",
			"Backup Files To Update", "Backup Files Updated", "Backup Files Not Updated Due To Errors",
			"Backup Bytes To Replicate (MB)", "Backup Bytes Replicated (MB)", "Backup Bytes Transferred (MB)",
			"Backup Bytes Not Replicated Due To Errors (MB)", "Backup Bytes Not Yet Replicated (MB)",
			"Archive Last Update Date/Time", "Archive Target Server", "Archive Files Needing No Action",
			"Archive Files To Replicate", "Archive Files Replicated", "Archive Files Not Replicated Due To Errors",
			"Archive Files Not Yet Replicated", "Archive Files To Delete", "Archive Files Deleted",
			"Archive Files Not Deleted Due To Errors", "Archive Files To Update", "Archive Files Updated",
			"Archive Files Not Updated Due To Errors", "Archive Bytes To Replicate (MB)",
			"Archive Bytes Replicated (MB)", "Archive Bytes Transferred (MB)",
			"Archive Bytes Not Replicated Due To Errors (MB)", "Archive Bytes Not Yet Replicated (MB)",
			"Space Managment Last Update Date/Time", "Space Managment Target Server",
			"Space Managed Files Needing No Action", "Space Managed Files To Replicate",
			"Space Managed Files Replicated", "Space Managed Files Not Replicated Due to Errors",
			"Space Managed Files Not Yet Replicated", "Space Managed Files To Delete",
			"Space Managed Files Deleted", "Space Managed Files Not Deleted Due To Errors",
			"Space Managed Files To Update", "Space Managed Files Updated",
			"Space Managed Files Not Updated Due To Errors", "Space Managed Bytes To Replicate (MB)",
			"Space Managed Bytes Replicated (MB)", "Space Managed Bytes Transferred (MB)",
			"Space Managed Bytes Not Replicated Due To Errors (MB)",
			"Space Managed Bytes Not Yet Replicated (MB)", "Total Files Needing No Action",
			"Total Files To Replicate", "Total Files Replicated", "Total Files Not Replicated Due To Errors",
			"Total Files Not Yet Replicated", "Total Files To Delete", "Total Files Deleted",
			"Total Files Not Detected Due To Errors", "Total Files To Update", "Total Files Updated",
			"Total Files Not Updated Due To Errors", "Total Bytes To Replicate (MB)",
			"Total Bytes Replicated (MB)", "Total Bytes Transferred (MB)",
			"Total Bytes Not Replicated Due To Errors (MB)", "Total Bytes Not Yet Replicated (MB)",
			"OSSM Replication", "Snapshots Needing No Action", "Snapshots To Replicate", "Snapshots Replicated",
			"Snapshots Not Replicated Due to Errors", "Snapshots Not Yet Replicated", "Estimated Percentage Complete",
			"Estimated Time Remaining", "Estimated Time Of Completion",
		}

	case "QUERY JOB STATUS=RUNNING":
		return []string{
			"Job Id", "Peer Job Id", "Job Type", "Job Name", "Begin Date/Time", "End Date/Time",
			"Run Date/Time", "Status",
		}

	case "QUERY SESSION FORMAT=DETAILED":
		return []string{
			"Sess Number", "Comm. Method", "Sess State", "Wait Time", "Bytes Sent", "Bytes Recvd",
			"Sess Type", "Platform", "Client Name", "Media Access Status", "User Name",
			"Date/Time First Data Sent", "Proxy By Storage Agent", "Actions", "Failover Mode",
		}

	default:
		return []string{}
	}
}
