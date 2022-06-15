# cron

*Based on github.com/robfig/cron/v3, but more convenient*

## Usaga

```go

package main

import (
    "fmt"
    "time"
    "github.com/kainhuck/cron"
)

func main() {
    croner := cron.NewCron()
    croner.Start(nil)

    croner.AddSecondJob(1, 1, hello(1), cron.ModeJobSerial)
    croner.AddSecondJob(2, 1, hello(2), cron.ModeTimeFirst)

    time.Sleep(10 * time.Second)
    croner.RemoveJob(1)
    croner.RemoveJob(2)
}

func hello(id int) func() {
    return func () {
        fmt.Println("hello", id)
        time.Sleep(2 * time.Second)
    }
}

```