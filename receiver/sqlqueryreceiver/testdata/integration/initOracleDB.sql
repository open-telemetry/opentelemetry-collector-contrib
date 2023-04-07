create table movie
(
    title       VARCHAR2(256),
    genre       VARCHAR(256),
    imdb_rating NUMBER
);

insert into movie (title, genre, imdb_rating)
values ('E.T.', 'SciFi', 7.9);
insert into movie (title, genre, imdb_rating)
values ('Blade Runner', 'SciFi', 8.1);
insert into movie (title, genre, imdb_rating)
values ('Star Wars', 'SciFi', 8.6);
insert into movie (title, genre, imdb_rating)
values ('Die Hard', 'Action', 8.2);
insert into movie (title, genre, imdb_rating)
values ('Mission Impossible', 'Action', 7.1);

/* The alter session command is required to enable user creation in an Oracle docker container
   This command shouldn't be used outside of test environments. */
alter session set "_ORACLE_SCRIPT"=true;
CREATE USER OTEL IDENTIFIED BY password;
GRANT CREATE SESSION TO OTEL;
GRANT ALL ON movie TO OTEL;

create table simple_logs
(
    id number primary key,
    body varchar2(4000)
);
grant select on simple_logs to otel;

insert into simple_logs (id, body) values
(1, '- - - [03/Jun/2022:21:59:26 +0000] "GET /api/health HTTP/1.1" 200 6197 4 "-" "-" 445af8e6c428303f -');
insert into simple_logs (id, body) values
(2, '- - - [03/Jun/2022:21:59:26 +0000] "GET /api/health HTTP/1.1" 200 6205 5 "-" "-" 3285f43cd4baa202 -');
insert into simple_logs (id, body) values
(3, '- - - [03/Jun/2022:21:59:29 +0000] "GET /api/health HTTP/1.1" 200 6233 4 "-" "-" 579e8362d3185b61 -');
insert into simple_logs (id, body) values
(4, '- - - [03/Jun/2022:21:59:31 +0000] "GET /api/health HTTP/1.1" 200 6207 5 "-" "-" 8c6ac61ae66e509f -');
insert into simple_logs (id, body) values
(5, '- - - [03/Jun/2022:21:59:31 +0000] "GET /api/health HTTP/1.1" 200 6200 4 "-" "-" c163495861e873d8 -');
