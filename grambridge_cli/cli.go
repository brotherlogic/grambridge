package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/brotherlogic/goserver/utils"

	pbrc "github.com/brotherlogic/recordcollection/proto"
)

func main() {
	ctx, cancel := utils.ManualContext("grambridge_cli", time.Minute*30)
	defer cancel()

	conn, err := utils.LFDialServer(ctx, "grambridge")
	if err != nil {
		log.Fatalf("Unable to dial: %v", err)
	}
	defer conn.Close()

	switch os.Args[1] {
	case "ping":
		id, _ := strconv.ParseInt(os.Args[2], 10, 32)
		sclient := pbrc.NewClientUpdateServiceClient(conn)
		r, err := sclient.ClientUpdate(ctx, &pbrc.ClientUpdateRequest{InstanceId: int32(id)})
		if err != nil {
			log.Fatalf("Error on GET: %v", err)
		}
		log.Printf("%v", r)

	}
}
