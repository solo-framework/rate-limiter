package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net"
	"sync"
	"time"
)

func main() {

	defer func() {
		fmt.Println("END!")
	}()

	groups := []string{"default", "users"}

	ctx, cf := context.WithTimeout(context.Background(), time.Second*60)
	defer cf()

	c := 0
	groupName := ""

	wg := sync.WaitGroup{}

	for {

		select {
		case <-ctx.Done():
			fmt.Println("TIMEOUT")
			// cf()
			return
		default:

			time.Sleep(time.Millisecond * 5)

			wg.Go(func() {

				time.Sleep(time.Millisecond * 100)

				tcpConn, err := net.Dial("tcp", "localhost:49105")
				if err != nil {
					log.Fatal(err)
				}
				defer tcpConn.Close()

				conn, ok := tcpConn.(*net.TCPConn)
				if !ok {
					log.Fatal("can't convert to *net.TCPConn")
				}

				groupName = groups[0]
				if c%2 == 0 {
					groupName = groups[1]
				}

				c++

				data := fmt.Sprintf("%s:user_%d", groupName, rand.Int32())

				_, _ = conn.Write([]byte(data))
				_ = conn.CloseWrite() // send EOF so server knows no more data is coming from client

				_, _ = io.ReadAll(conn) // read answer

				// fmt.Println(string(res))
			})
		}
	}

}
