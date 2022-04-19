CREATE
USER otel WITH PASSWORD 'otel';
create table movie
(
    title text,
    genre text
);
insert into movie (title, genre)
values ('E.T.', 'SciFi');
insert into movie (title, genre)
values ('Blade Runner', 'SciFi');
insert into movie (title, genre)
values ('Star Wars', 'SciFi');
insert into movie (title, genre)
values ('Die Hard', 'Action');
insert into movie (title, genre)
values ('Mission Impossible', 'Action');
grant select on movie to otel;
