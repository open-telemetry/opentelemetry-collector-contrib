-- Create some blocking sessions for testing query performance monitoring
USE TestDatabase;
GO

-- Create a procedure that will cause blocking for testing
CREATE PROCEDURE CreateBlockingSession
AS
BEGIN
    BEGIN TRANSACTION;
    
    -- This will hold a lock and cause blocking
    UPDATE Orders 
    SET Status = 'Blocked' 
    WHERE OrderID = 1;
    
    -- Wait for 30 seconds to create blocking
    WAITFOR DELAY '00:00:30';
    
    ROLLBACK TRANSACTION;
END
GO

-- Create a procedure to simulate long-running queries
CREATE PROCEDURE SimulateLongRunningQuery
AS
BEGIN
    -- Simulate a complex query that takes time
    WITH RecursiveCTE AS (
        SELECT 1 as Level, CustomerID, TotalAmount
        FROM Orders
        WHERE CustomerID = 1
        
        UNION ALL
        
        SELECT Level + 1, o.CustomerID, o.TotalAmount
        FROM Orders o
        INNER JOIN RecursiveCTE r ON o.CustomerID = r.CustomerID
        WHERE Level < 1000
    )
    SELECT Level, CustomerID, COUNT(*) as RecordCount
    FROM RecursiveCTE
    GROUP BY Level, CustomerID
    ORDER BY Level;
END
GO

-- Create a procedure to generate ongoing activity for metrics
CREATE PROCEDURE GenerateOngoingActivity
AS
BEGIN
    DECLARE @i int = 1;
    
    WHILE @i <= 100
    BEGIN
        -- Random SELECT operations
        SELECT TOP 10 * FROM Orders WHERE CustomerID = (@i % 5) + 1;
        
        -- Random UPDATE operations
        UPDATE Orders 
        SET Status = CASE (@i % 3) 
            WHEN 0 THEN 'Updated1'
            WHEN 1 THEN 'Updated2'
            ELSE 'Updated3'
        END
        WHERE OrderID = (@i % 8) + 1;
        
        -- Small delay
        WAITFOR DELAY '00:00:01';
        
        SET @i = @i + 1;
    END
END
GO

PRINT 'Test procedures for blocking and activity simulation created successfully';
GO
