package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/micaelapucciariello/simplebank/api"
	db "github.com/micaelapucciariello/simplebank/db/sqlc"
	"github.com/micaelapucciariello/simplebank/gapi"
	"github.com/micaelapucciariello/simplebank/pb"
	"github.com/micaelapucciariello/simplebank/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
	"net/http"

	_ "github.com/lib/pq"
)

func main() {
	cfg, err := utils.LoadConfig("")
	if err != nil {
		log.Fatal("cannot get config: ", err)
	}
	conn, err := sql.Open(cfg.DriverName, cfg.SourceName)
	if err != nil {
		log.Fatal(fmt.Sprintf("cannot connect to db: %s", err))
	}

	store := db.NewStore(conn)
	go runGatewayServer(cfg, store)
	rungRPCServer(cfg, store)
}

func runHTTPServer(cfg utils.Config, store db.Store) {
	server, err := api.NewServer(cfg, store)
	if err != nil {
		log.Fatal(fmt.Sprintf("cannot initiate http server: %s", err))
	}

	err = server.Start(cfg.HTTPServerAddress)
	if err != nil {
		log.Fatal(fmt.Sprintf("cannot start http server: %s", err))
	}
}

func rungRPCServer(cfg utils.Config, store db.Store) {
	server, err := gapi.NewServer(cfg, store)
	if err != nil {
		log.Fatal(fmt.Sprintf("cannot initiate gRPC server: %s", err))
	}
	grpcServer := grpc.NewServer()
	pb.RegisterSimpleBankServer(grpcServer, server)
	reflection.Register(grpcServer)

	listener, err := net.Listen("tcp", cfg.GRPCServerAddress)
	if err != nil {
		log.Fatal(fmt.Sprintf("cannot create listener: %s", err))
	}

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatal(fmt.Sprintf("cannot start gRPC server: %s", err))
	}

	log.Printf("gRPC server started at address %v", listener.Addr().String())
}

func runGatewayServer(cfg utils.Config, store db.Store) {
	server, err := gapi.NewServer(cfg, store)
	if err != nil {
		log.Fatal(fmt.Sprintf("cannot initiate gateway server: %s", err))
	}

	grpcMux := runtime.NewServeMux()

	// invokes the cancel context function when the execution is completed
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = pb.RegisterSimpleBankHandlerServer(ctx, grpcMux, server)
	if err != nil {
		log.Fatal(fmt.Sprintf("cannot create server handler: %s", err))
	}

	mux := http.NewServeMux()
	mux.Handle("/", grpcMux)

	listener, err := net.Listen("tcp", cfg.HTTPServerAddress)
	if err != nil {
		log.Fatal(fmt.Sprintf("cannot create listener: %s", err))
	}
	log.Printf("HTTP Gateway server listening at address %v", listener.Addr().String())
	err = http.Serve(listener, mux)
	if err != nil {
		log.Fatal(fmt.Sprintf("cannot start HTTP Gateway server: %s", err))
	}

	log.Printf("HTTP Gateway server started at address %v", listener.Addr().String())
}
