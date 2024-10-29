use otel;

create table movie
(
    title       text,
    genre       text,
    imdb_rating float
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

create table simple_logs
(
    id integer,
    insert_time timestamp,
    body text,
    attribute text,
    primary key (id)
);

insert into simple_logs (id, insert_time, body, attribute) values
(1, '2022-06-03 21:59:26', '- - - [03/Jun/2022:21:59:26 +0000] "GET /api/health HTTP/1.1" 200 6197 4 "-" "-" 445af8e6c428303f -', 'TLSv1.2'),
(2, '2022-06-03 21:59:26.692991', '- - - [03/Jun/2022:21:59:26 +0000] "GET /api/health HTTP/1.1" 200 6205 5 "-" "-" 3285f43cd4baa202 -', 'TLSv1'),
(3, '2022-06-03 21:59:29.212212', '- - - [03/Jun/2022:21:59:29 +0000] "GET /api/health HTTP/1.1" 200 6233 4 "-" "-" 579e8362d3185b61 -', 'TLSv1.2'),
(4, '2022-06-03 21:59:31', '- - - [03/Jun/2022:21:59:31 +0000] "GET /api/health HTTP/1.1" 200 6207 5 "-" "-" 8c6ac61ae66e509f -', 'TLSv1'),
(5, '2022-06-03 21:59:31.332121', '- - - [03/Jun/2022:21:59:31 +0000] "GET /api/health HTTP/1.1" 200 6200 4 "-" "-" c163495861e873d8 -', 'TLSv1.2');
