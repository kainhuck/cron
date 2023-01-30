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
	crond := cron.NewCron()
	crond.Start(nil)

	hello1 := crond.AddSecondJob(1, hello(1), cron.Option{
		RunMode: cron.ModeJobSerial,
	})
	hello2 := crond.AddSecondJob(1, hello(2), cron.Option{
		RunMode: cron.ModeTimeFirst,
	})

	time.Sleep(10 * time.Second)
	crond.Call(hello1)
	crond.RemoveJob(hello2)
	time.Sleep(10 * time.Second)
	crond.RemoveJob(hello1)
}

func hello(id int) func() {
	return func() {
		fmt.Println("hello", id)
		time.Sleep(2 * time.Second)
	}
}

```