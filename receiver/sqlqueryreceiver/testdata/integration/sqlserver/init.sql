USE master;
GO

CREATE DATABASE otel;
GO

CREATE LOGIN otel WITH PASSWORD = 'YourStrong!Passw0rd';
GO

USE otel;
GO

CREATE USER otel FOR LOGIN otel;
GO

ALTER ROLE db_owner ADD MEMBER otel;
GO

CREATE TABLE movie
(
    title NVARCHAR(256),
    genre NVARCHAR(256),
    imdb_rating FLOAT
);

PRINT 'Inserting data into movie table...';
INSERT INTO movie (title, genre, imdb_rating)
VALUES ('E.T.', 'SciFi', 7.9),
       ('Blade Runner', 'SciFi', 8.1),
       ('Star Wars', 'SciFi', 8.6),
       ('Die Hard', 'Action', 8.2),
       ('Mission Impossible', 'Action', 7.1);
PRINT 'Data inserted into movie table.';

CREATE TABLE simple_logs
(
    id INT PRIMARY KEY,
    insert_time DATETIME2 default GETDATE(),
    body NVARCHAR(MAX),
    attribute NVARCHAR(100)
);

PRINT 'Inserting data into simple_logs table...';
INSERT INTO simple_logs (id, insert_time, body, attribute) VALUES
                                                               (1, '2022-06-03 21:59:26', '- - - [03/Jun/2022:21:59:26 +0000] "GET /api/health HTTP/1.1" 200 6197 4 "-" "-" 445af8e6c428303f -', 'TLSv1.2'),
                                                               (2, '2022-06-03 21:59:26.692991', '- - - [03/Jun/2022:21:59:26 +0000] "GET /api/health HTTP/1.1" 200 6205 5 "-" "-" 3285f43cd4baa202 -', 'TLSv1'),
                                                               (3, '2022-06-03 21:59:29.212212', '- - - [03/Jun/2022:21:59:29 +0000] "GET /api/health HTTP/1.1" 200 6233 4 "-" "-" 579e8362d3185b61 -', 'TLSv1.2'),
                                                               (4, '2022-06-03 21:59:31', '- - - [03/Jun/2022:21:59:31 +0000] "GET /api/health HTTP/1.1" 200 6207 5 "-" "-" 8c6ac61ae66e509f -', 'TLSv1'),
                                                               (5, '2022-06-03 21:59:31.332121', '- - - [03/Jun/2022:21:59:31 +0000] "GET /api/health HTTP/1.1" 200 6200 4 "-" "-" c163495861e873d8 -', 'TLSv1.2');
PRINT 'Data inserted into simple_logs table.';
PRINT 'Initiation of otel database is complete';