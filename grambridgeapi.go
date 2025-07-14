package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pbgd "github.com/brotherlogic/godiscogs/proto"
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

	resp, err := gclient.RefreshRecord(nctx, &pbg.RefreshRecordRequest{InstanceId: int64(req.GetInstanceId()), JustState: true})

	s.CtxLog(ctx, fmt.Sprintf("Refreshed %v -> %v", req.GetInstanceId(), err))

	// AlreadyExists should be a soft error - swallow this ehre
	if status.Code(err) == codes.AlreadyExists {
		return &rcpb.ClientUpdateResponse{}, nil
	}

	conn, err = s.FDialServer(ctx, "recordcollection")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := rcpb.NewRecordCollectionServiceClient(conn)
	rec, err := client.GetRecord(ctx, &rcpb.GetRecordRequest{InstanceId: req.GetInstanceId()})
	if err != nil {
		s.CtxLog(ctx, fmt.Sprintf("Error getting record %v: %v", req.GetInstanceId(), err))
		return nil, err
	}

	if resp.GetSaleId() > 0 || resp.GetHighPrice() != rec.GetRecord().GetMetadata().GetHighPrice() {
		s.CtxLog(ctx, fmt.Sprintf("Updating %v", rec.GetRecord()))
		_, err = client.UpdateRecord(ctx, &rcpb.UpdateRecordRequest{
			Reason: "updating from grambridge",
			Update: &rcpb.Record{
				Release: &pbgd.Release{InstanceId: req.GetInstanceId()},
				Metadata: &rcpb.ReleaseMetadata{
					SaleId:    resp.GetSaleId(),
					HighPrice: resp.GetHighPrice(),
				}}})
		return &rcpb.ClientUpdateResponse{}, err

	}

	return &rcpb.ClientUpdateResponse{}, err
}
