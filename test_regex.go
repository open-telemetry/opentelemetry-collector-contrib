package main

import (
    "fmt"
    "regexp"
)

func main() {
    // Test invalid pattern
    _, err := regexp.Compile("*.")
    if err != nil {
        fmt.Printf("Invalid regex error: %v\n", err)
    }

    // Test valid pattern
    _, err2 := regexp.Compile(".*")
    if err2 != nil {
        fmt.Printf("Invalid regex error: %v\n", err2)
    } else {
        fmt.Println("Valid regex matches everything")
    }
}