# frozenDB Get Function Quickstart

This quickstart demonstrates how to use the Get function to retrieve data from frozenDB.

## Basic Usage

```go
package main

import (
    "fmt"
    "github.com/google/uuid"
    "github.com/anilcode/frozenDB/frozendb"
)

func main() {
    // Open database
    db, err := frozendb.Open("mydb.fdb")
    if err != nil {
        panic(err)
    }
    defer db.Close()
    
    // Define destination struct
    type User struct {
        Name string `json:"name"`
        Age  int    `json:"age"`
    }
    
    var user User
    
    // Retrieve user by UUID
    userID := uuid.MustParse("018f5d8b-4a5b-7c8d-9e0f-123456789abc")
    err = db.Get(userID, &user)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("User: %s, Age: %d\n", user.Name, user.Age)
}
```

## Error Handling

```go
var user User
err := db.Get(userID, &user)

switch {
case errors.Is(err, &frozendb.KeyNotFoundError{}):
    fmt.Println("User not found")
case errors.Is(err, &frozendb.InvalidDataError{}):
    fmt.Println("Invalid JSON data format")
case errors.Is(err, &frozendb.InvalidInputError{}):
    fmt.Println("Invalid input parameters")
case err != nil:
    fmt.Printf("Database error: %v\n", err)
default:
    fmt.Printf("Successfully retrieved: %+v\n", user)
}
```

## Nested JSON

```go
type Profile struct {
    Bio    string `json:"bio"`
    Active bool   `json:"active"`
}

type User struct {
    ID      int     `json:"id"`
    Name    string  `json:"name"`
    Profile Profile `json:"profile"`
}

var user User
err := db.Get(userID, &user)
if err != nil {
    // Handle error
}

fmt.Printf("User: %s, Bio: %s, Active: %t\n", 
    user.Name, user.Profile.Bio, user.Profile.Active)
```

## Key Points

- Always pass a pointer to `Get()` - never a value
- Use proper error handling to distinguish between missing keys and data corruption
- The destination type must match the stored JSON structure
- Get() only returns data from committed transactions