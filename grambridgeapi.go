package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pbg "github.com/brotherlogic/gramophile/proto"
	rcpb "github.com/brotherlogic/recordcollection/proto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	gramError = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "grambridge_error",
		Help: "The size of the print queue",
	}, []string{"code"})

	cacheSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "grambridge_cache",
		Help: "The size of the gram cache",
	})
)

func buildContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, err
	}

	text, err := ioutil.ReadFile(fmt.Sprintf("%v/.gramophile", dirname))
	if err != nil {
		return nil, nil, err
	}

	user := &pbg.GramophileAuth{}
	err = proto.UnmarshalText(string(text), user)
	if err != nil {
		return nil, nil, err
	}

	mContext := metadata.AppendToOutgoingContext(ctx, "auth-token", user.GetToken())
	ctx, cancel := context.WithTimeout(mContext, time.Minute)
	return ctx, cancel, nil
}

// ClientUpdate on an updated record
func (s *Server) ClientUpdate(ctx context.Context, req *rcpb.ClientUpdateRequest) (*rcpb.ClientUpdateResponse, error) {
	// Dial gram
	conn, err := grpc.Dial("gramophile-grpc.brotherlogic-backend.com:80", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	gclient := pbg.NewGramophileEServiceClient(conn)
	nctx, cancel, gerr := buildContext(ctx)
	if gerr != nil {
		return nil, gerr
	}
	defer cancel()

	_, err = gclient.RefreshRecord(nctx, &pbg.RefreshRecordRequest{InstanceId: int64(req.GetInstanceId())})

	return &rcpb.ClientUpdateResponse{}, err
}
