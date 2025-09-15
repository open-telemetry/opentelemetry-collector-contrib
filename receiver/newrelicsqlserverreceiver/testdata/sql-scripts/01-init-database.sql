-- Initialize test database for New Relic SQL Server receiver testing
USE master;
GO

-- Create test database
IF NOT EXISTS (SELECT name FROM sys.databases WHERE name = 'TestDatabase')
BEGIN
    CREATE DATABASE TestDatabase;
END
GO

USE TestDatabase;
GO

-- Create test tables with some data to generate metrics
CREATE TABLE Orders (
    OrderID int IDENTITY(1,1) PRIMARY KEY,
    CustomerID int NOT NULL,
    OrderDate datetime2 DEFAULT GETDATE(),
    TotalAmount decimal(10,2) NOT NULL,
    Status varchar(50) DEFAULT 'Pending'
);
GO

CREATE TABLE Customers (
    CustomerID int IDENTITY(1,1) PRIMARY KEY,
    FirstName varchar(50) NOT NULL,
    LastName varchar(50) NOT NULL,
    Email varchar(100) UNIQUE,
    CreatedDate datetime2 DEFAULT GETDATE()
);
GO

-- Create indexes for testing
CREATE INDEX IX_Orders_CustomerID ON Orders(CustomerID);
CREATE INDEX IX_Orders_OrderDate ON Orders(OrderDate);
CREATE INDEX IX_Customers_Email ON Customers(Email);
GO

-- Insert test data
INSERT INTO Customers (FirstName, LastName, Email) VALUES
('John', 'Doe', 'john.doe@example.com'),
('Jane', 'Smith', 'jane.smith@example.com'),
('Bob', 'Johnson', 'bob.johnson@example.com'),
('Alice', 'Brown', 'alice.brown@example.com'),
('Charlie', 'Wilson', 'charlie.wilson@example.com');
GO

INSERT INTO Orders (CustomerID, TotalAmount, Status) VALUES
(1, 150.00, 'Completed'),
(2, 75.50, 'Pending'),
(3, 200.00, 'Completed'),
(1, 85.00, 'Processing'),
(4, 320.00, 'Completed'),
(5, 45.75, 'Pending'),
(2, 180.00, 'Completed'),
(3, 95.25, 'Processing');
GO

-- Create a stored procedure that will generate some activity
CREATE PROCEDURE GenerateTestActivity
AS
BEGIN
    DECLARE @i int = 1;
    DECLARE @CustomerID int;
    DECLARE @Amount decimal(10,2);
    
    WHILE @i <= 10
    BEGIN
        SET @CustomerID = (SELECT TOP 1 CustomerID FROM Customers ORDER BY NEWID());
        SET @Amount = RAND() * 500 + 10;
        
        INSERT INTO Orders (CustomerID, TotalAmount, Status)
        VALUES (@CustomerID, @Amount, 'Generated');
        
        SET @i = @i + 1;
    END
END
GO

-- Create a view for testing
CREATE VIEW CustomerOrderSummary AS
SELECT 
    c.CustomerID,
    c.FirstName + ' ' + c.LastName as CustomerName,
    COUNT(o.OrderID) as TotalOrders,
    SUM(o.TotalAmount) as TotalSpent,
    AVG(o.TotalAmount) as AvgOrderValue
FROM Customers c
LEFT JOIN Orders o ON c.CustomerID = o.CustomerID
GROUP BY c.CustomerID, c.FirstName, c.LastName;
GO

PRINT 'Test database and data initialized successfully';
GO
