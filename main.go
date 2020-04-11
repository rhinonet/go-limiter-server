/*
 *
 * Copyright 2015 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

//go:generate protoc -I ../helloworld --go_out=plugins=grpc:../helloworld ../helloworld/helloworld.proto

// Package main implements a server for Greeter service.
package main

import (
	"fmt"
	"golang.org/x/net/context"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	pb "google.golang.org/grpc/examples/go_limiter/pb"
	lib "google.golang.org/grpc/examples/go_limiter/lib"
	"github.com/go-redis/redis"
	"strconv"
	"time"
	"encoding/xml"

)

const (
	port = ":50053"
)

// server is used to implement helloworld.GreeterServer.
type server struct {
	//pb.UnimplementedGreeterServer
       pb.UnimplementedRateLimiterServer
}


var client *redis.Client

type configuration struct {
	Addr string
	Password string
	Db string
}


func (s *server) Create(ctx context.Context, in *pb.CreateRequest) (*pb.CreateReply, error) {
	var alias string = in.GetAlias()
	var permits int =  int(in.GetPermits())

	_, err := client.Ping().Result()
	if err != nil {
		return &pb.CreateReply{Ret:false},nil
		//panic(err)
	}

	var per = "permits:" + alias
	val, err := client.Get(per).Result()
	if err != nil {
		if err == redis.Nil {
			//register
			err1 := client.Set(per, permits, 0).Err()
			if err1 != nil {
				return &pb.CreateReply{Ret:false},nil
				//panic(err)
			}

			return &pb.CreateReply{Ret:true},nil
		}else{
			return &pb.CreateReply{Ret:false},nil
			//panic(err)
		}
	}else{
		ori ,err2 := strconv.Atoi(val)
		if err2 != nil {
			return &pb.CreateReply{Ret:false},nil
			//panic(err)
		}
		if ori != permits {
			err3 := client.Set(per, permits, 0).Err()
			if err3 != nil {
				return &pb.CreateReply{Ret:false},nil
				//panic(err)
			}
		}
	}

	return &pb.CreateReply{Ret:true},nil
}


func (s *server) GetRate(ctx context.Context, in *pb.GetRateRequest) (*pb.GetRateReply, error) {
	var alias string = in.GetAlias()

	_, err := client.Ping().Result()
	if err != nil {
		return &pb.GetRateReply{Ret:false, Permits:0},nil
		//panic(err)
	}

	var volume_key = "volume:" + alias
	val, err := client.Get(volume_key).Result()
	if err != nil {
		return &pb.GetRateReply{Ret:false, Permits:0},nil
	}

	vv , _  := strconv.Atoi(val)
	return &pb.GetRateReply{Ret:true, Permits:int32(vv)},nil
}

func (s *server) Acquire(ctx context.Context, in *pb.AcquireRequest) (*pb.AcquireReply, error) {
	var alias string = in.GetAlias()
	var timeout = int(in.GetTimeout())

	_, err := client.Ping().Result()
	if err != nil {
		return &pb.AcquireReply{Ret:false},nil
		//panic(err)
	}
	var per = "permits:" + alias
	val, err := client.Get(per).Result()

	if err != nil {
		return &pb.AcquireReply{Ret:false},nil
		//panic(err)
	}
	ori ,err2 := strconv.Atoi(val)
	if err2 != nil {
		return &pb.AcquireReply{Ret:false},nil
		//panic(err2)
	}

	limiter1 := lib.NewLimiter(client)

	var create_spent float32 = (1 /  float32(ori))
	var i = int(float32(timeout) / create_spent)
	for {
		if(i <= 1){
			break
		}
		res, err := limiter1.AllowN(alias, lib.PerSecond(ori), 1)
		if err != nil {
			panic(err)
		}
		if(res.Allowed){
			return &pb.AcquireReply{Ret:true},nil
		}else{
			time.Sleep(time.Duration(create_spent*1000) * time.Millisecond)
		}
		i--
	}

	return &pb.AcquireReply{Ret:false},nil
}


func (s *server) TryAcquire(ctx context.Context, in *pb.TryAcquireRequest) (*pb.TryAcquireReply, error) {
	var alias string = in.GetAlias()

	_, err := client.Ping().Result()
	if err != nil {
		return &pb.TryAcquireReply{Ret:false},nil
		//panic(err)
	}

	var per = "permits:" + alias
	val, err := client.Get(per).Result()

	if err != nil {
		return &pb.TryAcquireReply{Ret:false},nil
		//panic(err)
	}


	ori ,err2 := strconv.Atoi(val)
	if err2 != nil {
		return &pb.TryAcquireReply{Ret:false},nil
		//panic(err2)
	}

	limiter1 := lib.NewLimiter(client)
	res, err := limiter1.AllowN(alias, lib.PerSecond(ori), 1)
	if err != nil {
		panic(err)
	}

	return &pb.TryAcquireReply{Ret:res.Allowed},nil
}



func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	xmlFile, err := os.Open("conf.xml")
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer xmlFile.Close()

	var conf configuration
	if err := xml.NewDecoder(xmlFile).Decode(&conf); err != nil {
		fmt.Println("Error Decode file:", err)
		return
	}

	client = redis.NewClient(&redis.Options{
		Addr:     conf.Addr,
		Password: conf.Password, // no password set
	})

	s := grpc.NewServer()
	pb.RegisterRateLimiterServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
