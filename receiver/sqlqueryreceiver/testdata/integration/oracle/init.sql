/* The alter session command is required to enable user creation in an Oracle docker container
   This command shouldn't be used outside of test environments. */
alter session set "_ORACLE_SCRIPT"=true;
ALTER SESSION SET CONTAINER=FREEPDB1;
CREATE USER OTEL IDENTIFIED BY otel QUOTA UNLIMITED ON USERS
GRANT CONNECT, RESOURCE TO OTEL;
-- Switch to the OTEL schema
ALTER SESSION SET CURRENT_SCHEMA = OTEL;
      
create table movie
(
    title       VARCHAR2(256),
    genre       VARCHAR(256),
    imdb_rating NUMBER
);
GRANT ALL ON movie TO OTEL;

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

create table simple_logs
(
    id number primary key,
    insert_time timestamp with time zone default SYSTIMESTAMP,
    body varchar2(4000),
    attribute varchar2(100)
);
GRANT ALL ON simple_logs TO OTEL;

insert into simple_logs (id, insert_time, body, attribute) values
(1, TIMESTAMP '2022-06-03 21:59:26 +00:00', '- - - [03/Jun/2022:21:59:26 +0000] "GET /api/health HTTP/1.1" 200 6197 4 "-" "-" 445af8e6c428303f -', 'TLSv1.2');
insert into simple_logs (id, insert_time, body, attribute) values
(2, TIMESTAMP '2022-06-03 21:59:26.692991 +00:00', '- - - [03/Jun/2022:21:59:26 +0000] "GET /api/health HTTP/1.1" 200 6205 5 "-" "-" 3285f43cd4baa202 -', 'TLSv1');
insert into simple_logs (id, insert_time, body, attribute) values
(3, TIMESTAMP '2022-06-03 21:59:29.212212 +00:00', '- - - [03/Jun/2022:21:59:29 +0000] "GET /api/health HTTP/1.1" 200 6233 4 "-" "-" 579e8362d3185b61 -', 'TLSv1.2');
insert into simple_logs (id, insert_time, body, attribute) values
(4, TIMESTAMP '2022-06-03 21:59:31 +00:00', '- - - [03/Jun/2022:21:59:31 +0000] "GET /api/health HTTP/1.1" 200 6207 5 "-" "-" 8c6ac61ae66e509f -', 'TLSv1');
insert into simple_logs (id, insert_time, body, attribute) values
(5, TIMESTAMP '2022-06-03 21:59:31.332121 +00:00', '- - - [03/Jun/2022:21:59:31 +0000] "GET /api/health HTTP/1.1" 200 6200 4 "-" "-" c163495861e873d8 -', 'TLSv1.2');
