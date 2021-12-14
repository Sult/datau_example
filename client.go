package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/ice2heart/proxyu_client/common"

	pb "github.com/ice2heart/proxyu_client/protocol"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var (
	keyPath       = flag.String("tls-key", "client.dev.key", "Path to TLS private key")
	certPath      = flag.String("tls-cert", "client.dev.pem", "Path to TLS certificate (if using TCP)")
	rootCertPath  = flag.String("tls-ca-cert", "ca.pem", "Path to TLS CA root certificate (if using TCP)")
	proxyuAddress = flag.String("proxyu", "proxyu:8080", "ProxyU fqdn:port")
	userDataDB    = flag.String("userdata", "userdata.db", "File to store userdata")
	processUUID   = flag.String("process", "d31572a0-3799-4391-b3ac-149537a29b38", "UUID of process")
	dagyml        = flag.String("dag", "didgraph.yml", "Path to dag description file")
	dagdevyml     = flag.String("dag-dev", "./l10n/dev.yml", "Path to the translation file")
	serverPort    = flag.Int("port", 8090, "Web server port")
	dataUUIDs     map[string][]byte
	proxyuClient  pb.ProxyUIntegrationClient
)

// Directory contain files for html template
type Directory struct {
	Files []string
}

type AuthStatus struct {
	Status bool `json:"status" gorm:"not null"`
}

type dataRequest struct {
	Request  *pb.DataRequest_RetrieveRequest
	Response chan *pb.DataField
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if v := recover(); v != nil {
			cancel()
		}
	}()

	flag.Parse()
	// Parse graph of type of data.
	ParseDAGYML(dagyml)
	dataUUIDs = ParseDAGDevYML(dagdevyml)

	// Catch signals and close listener socket
	intCh := make(chan os.Signal, 1)
	signal.Notify(intCh, os.Interrupt, syscall.SIGTERM)

	// change to ctx func
	go OpenDB(userDataDB, intCh)

	conn, err := grpc.Dial(*proxyuAddress, grpc.WithTransportCredentials(common.LoadTLSKeys(certPath, keyPath, rootCertPath)))
	if err != nil {
		logrus.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	proxyuClient = pb.NewProxyUIntegrationClient(conn)

	dataProcessingChanel := make(chan *dataRequest)
	go dataProcessing(ctx, cancel, proxyuClient, dataProcessingChanel)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Route("/api", func(r chi.Router) {
		// r.Post("/login", createArticle)                                        // POST /articles
		r.Get("/login", getLogin) // GET /articles/search
		r.Get("/auth", HandleAuth(proxyuClient))
		r.Get("/request/{id:[0-9a-f-]+}", HandleRequest(proxyuClient))
		r.Get("/dag", getDAG)
		r.Route("/user", func(r chi.Router) {
			r.Get("/permissions", makeGetPermission(dataProcessingChanel))
		})
	})

	server := &http.Server{Addr: fmt.Sprintf(":%d", *serverPort), Handler: r}

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			logrus.Info("HTTP Server Error - ", err)
		}
	}()

	select {
	case <-ctx.Done():
		logrus.Info("Global shutdown")
	case <-intCh:
		logrus.Info("Gracefully stopping... (press Ctrl+C again to force)")
	}

	cancel()
	server.Shutdown(ctx)

	time.Sleep(100 * time.Microsecond)

}

type CorrellationMessage struct {
	Done    bool   `json:"done"`
	Message string `json:"msg"`
	Pubkey  string `json:"-"`
}

func HandleAuth(client pb.ProxyUIntegrationClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("userUUID")
		if err != nil {
			logrus.Panic(err)
		}
		var userUUID [16]byte
		copy(userUUID[:], common.S2B(cookie.Value))

		ctx := r.Context()
		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		// h.Set("X-Accel-Buffering", "no")

		f, ok := w.(http.Flusher)
		if !ok {
			return
		}

		data := make(chan CorrellationMessage)
		go func(globalCtx context.Context, data chan CorrellationMessage) {
			defer close(data)
			ctx, cancel := context.WithCancel(globalCtx)
			defer cancel()
			stream, err := client.Correlation(ctx, &pb.CorrelationRequest{})
			if err != nil {
				// Error report
				logrus.Error(err)
				return
			}
			defer stream.CloseSend()

			for {
				in, err := stream.Recv()
				if err == io.EOF {
					// func closed no more data
					return
				}
				if err != nil {
					logrus.Error("HandleAuth Failed to receive a note : %v", err)
					return
				}
				// use struct
				switch u := in.GetResponse().(type) {
				case *pb.CorrelationResponse_CorrelationMessage:
					data <- CorrellationMessage{Message: u.CorrelationMessage, Done: false, Pubkey: ""}
				case *pb.CorrelationResponse_PublicKey:
					data <- CorrellationMessage{Message: "", Done: true, Pubkey: common.B2S(u.PublicKey)}
				}
			}

		}(ctx, data)

		// f.Flush()
	L:
		for {
			select {
			case <-ctx.Done():
				logrus.Info("events: stream cancelled")
				break L
			case d, ok := <-data:
				if !ok {
					break L
				}
				e, _ := json.Marshal(&d)
				// logrus.Info("Done status :", d)
				if d.Done {
					var pubKey [32]byte
					copy(pubKey[:], common.S2B(d.Pubkey))
					WriteSession(&userUUID, &pubKey)
					logrus.Info("Pubkey ", d.Pubkey)
				}
				io.WriteString(w, "data: ")
				io.WriteString(w, string(e))
				io.WriteString(w, "\nevent: Login\n\n\n")
				logrus.Info("Write data", d)
				f.Flush()
			}
		}
		f.Flush()
		logrus.Info("events: stream closed")
	}
}

func HandleRequest(client pb.ProxyUIntegrationClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("userUUID")
		if err != nil {
			logrus.Panic(err)
		}
		var userUUID [16]byte
		copy(userUUID[:], common.S2B(cookie.Value))
		pubKey := GetSession(&userUUID)
		ctx := r.Context()
		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		h.Set("X-Accel-Buffering", "no")

		f, ok := w.(http.Flusher)
		if !ok {
			return
		}

		// ToDo: make params
		logrus.Info("Pubkey", pubKey)
		dataID := chi.URLParam(r, "id")
		// dataType := "NAME"
		reason := common.UUID2bytes("323fd1ea-76c7-4069-8fb1-d223f816c927")
		// User real hash
		policy := common.S2B("zqNzKjKy2SUf4SR+dGlLeBfHKaCWPBc6jKANOOM5XAY=")
		from := uint64(1605087413)
		until := uint64(1893456000)
		amount := uint32(0)
		level := uint32(1)
		message := &pb.PermissionRequest{
			Amount: amount,
			// Data:      dataUUIDs[dataType],
			Data:      common.UUID2bytes(dataID),
			From:      from,
			Level:     level,
			Policy:    policy,
			Process:   common.UUID2bytes(*processUUID),
			PublicKey: pubKey,
			Reason:    reason,
			Until:     until,
		}

		data := make(chan CorrellationMessage)
		go func(globalCtx context.Context, message *pb.PermissionRequest, data chan CorrellationMessage) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stream, err := client.Permission(ctx, message)
			if err != nil {
				logrus.Panic(err)
			}
			defer stream.CloseSend()
			for {
				in, err := stream.Recv()
				if err == io.EOF {
					// read done.
					return
				}
				if err != nil {
					logrus.Fatalf("Failed to receive a note : %v", err)
					return
				}
				switch u := in.GetResponse().(type) {
				case *pb.PermissionResponse_Granted:
					var empty [1]byte
					s := "Empty"
					var key [32]byte
					var d [16]byte
					copy(key[:], message.PublicKey)
					copy(d[:], message.Data)
					WriteUserData(&key, &d, &s, empty[:])
					data <- CorrellationMessage{Message: "", Done: u.Granted, Pubkey: ""}
				case *pb.PermissionResponse_PermissionMessage:
					data <- CorrellationMessage{Message: u.PermissionMessage, Done: false, Pubkey: ""}
				}
			}
		}(ctx, message, data)

		f.Flush()
	L:
		for {
			select {
			case <-ctx.Done():
				logrus.Info("events: stream cancelled")
				break L
			case d, ok := <-data:
				if !ok {
					break L
				}
				e, _ := json.Marshal(&d)
				if d.Done {
					logrus.Info("Permission granted")
				}
				io.WriteString(w, "event: Permission\ndata: ")
				io.WriteString(w, string(e))
				io.WriteString(w, "\n\n")
				logrus.Info("Write data", d)
				f.Flush()
			}
		}
		f.Flush()
		logrus.Info("events: stream closed")
	}
}

func getLogin(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("userUUID")
	if err != nil || cookie.Value == "" {
		expiration := time.Now().Add(365 * 24 * time.Hour)
		newUUID := common.RandomUUID()
		cookie = &http.Cookie{Name: "userUUID", Value: common.B2S(newUUID), Expires: expiration}
	}
	var userUUID [16]byte
	copy(userUUID[:], common.S2B(cookie.Value))
	session := GetSession(&userUUID)
	if session != nil {
		logrus.Info("authenticated")
		w.WriteHeader(http.StatusOK)
		status := AuthStatus{Status: true}
		http.SetCookie(w, cookie)
		render.JSON(w, r, status)
		return
	}
	http.SetCookie(w, cookie)
	w.WriteHeader(http.StatusAccepted)
	status := AuthStatus{Status: false}
	render.JSON(w, r, status)
}

func getDAG(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, daggraph)
}

type permissionMessage struct {
	Status int32  `json:"status"`
	Value  []byte `json:"value, string"`
}

func makeGetPermission(dataReq chan *dataRequest) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// var data map[string]permissionMessage
		cookie, err := r.Cookie("userUUID")
		if err != nil {
			logrus.Panic(err)
		}
		var userUUID [16]byte
		copy(userUUID[:], common.S2B(cookie.Value))
		var pubKey [32]byte
		copy(pubKey[:], GetSession(&userUUID))
		records, err := GetAllUserData(&pubKey)
		if err != nil {
			logrus.Panic(err)
		}
		data := make(map[string]permissionMessage)
		for _, record := range records {
			u, err := uuid.FromBytes(record.Data[:])
			if err != nil {
				logrus.Error(err)
			}
			status := 1
			logrus.Printf("uuid %v - %s", record, u.String())
			data[u.String()] = permissionMessage{Status: int32(status), Value: record.Value}
			if record.Mime == "Empty" {
				status = 0
				req := &pb.DataRequest_RetrieveRequest{
					RetrieveRequest: &pb.DataRetrieveRequest{
						Data:      record.Data[:],
						Process:   common.UUID2bytes(*processUUID),
						PublicKey: pubKey[:],
					},
				}
				resp := make(chan *pb.DataField)
				r := &dataRequest{Request: req, Response: resp}
				dataReq <- r
				for msg := range resp {
					logrus.Info("Get msg")
					u, _ := uuid.FromBytes(msg.Uuid)
					data[u.String()] = permissionMessage{Status: 2, Value: msg.GetValue()}
				}
				logrus.Info("Message end")
			}

		}

		// data["ab493ade-2f3f-11eb-a11b-23fff9ac0d99"] = permissionMessage{Status: 1, Value: []byte("Albert")}
		render.JSON(w, r, data)
	}
}

func dataProcessing(globCtx context.Context, globCancel context.CancelFunc, client pb.ProxyUIntegrationClient, dataReq chan *dataRequest) {
	defer func() {
		if v := recover(); v != nil {
			globCancel()
		}
	}()
	logrus.Info("Prepare data")
	ctx, cancel := context.WithCancel(globCtx)
	defer cancel()
	stream, err := client.Data(ctx)
	if err != nil {
		logrus.Panic(err)
	}

	respChan := make(map[[48]byte]chan *pb.DataField)
	waitChanel := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				logrus.Info("EOF")
				close(waitChanel)
				return
			}
			if err != nil {
				logrus.Fatalf("Failed to receive a message : %v", err)
				close(waitChanel)
				return
			}
			switch u := in.GetResponse().(type) {
			case *pb.DataResponse_RetrieveResponse:
				{
					if u.RetrieveResponse.GetError() != 0 {
						continue
					}
					values := ""

					line := fmt.Sprintf("%v %v", common.Bytes2uuid(u.RetrieveResponse.GetData()), values)
					// вот тут надо вытащить ответный канал и положить туда данные

					logrus.Println("Retrive data", line)
					var key [48]byte
					copy(key[:], u.RetrieveResponse.PublicKey)
					copy(key[32:], u.RetrieveResponse.Data)

					for _, f := range u.RetrieveResponse.GetFields() {
						logrus.Printf("Get fields %v", f)
						respChan[key] <- f
						logrus.Info(f)
					}

					close(respChan[key])
					delete(respChan, key)

				}
			case *pb.DataResponse_RetrieveRequest:
				{
					var pubKey [32]byte
					copy(pubKey[:], u.RetrieveRequest.GetPublicKey())
					var dataUUID [16]byte
					copy(dataUUID[:], u.RetrieveRequest.GetData())
					var process [16]byte
					copy(process[:], u.RetrieveRequest.GetProcess())
					data, mime := ExtractUserData(&pubKey, &dataUUID)
					if data == nil {
						// FOR DEBUG PURPURE
						res := []byte("NONE")
						data = make([]byte, len(res))
						copy(data, res)
						mime = "application/datau+node"
					}
					childrenID := GetDAGChildren(&dataUUID)
					var fields []*pb.DataField
					fields = make([]*pb.DataField, 0)
					for _, child := range childrenID {
						chdata, chmime := ExtractUserData(&pubKey, &child)
						logrus.Printf("Extracted user data: Mime: %v, uuid: %v, value: %v", chmime, common.Bytes2uuid(child[:]), chdata)
						if chdata == nil {
							// FOR DEBUG PURPURE
							res := []byte("NONE")
							chdata = make([]byte, len(res))
							copy(chdata, res)
							chmime = "text/plain; charset=UTF-8"
							logrus.Printf("Placeholder for user data: Mime: %v, uuid: %v, value: %v", chmime, common.Bytes2uuid(child[:]), chdata)
						}
						chuuid := child // without copy for all fields will be a copy of last slice.
						fields = append(fields, &pb.DataField{
							Mime:  chmime,
							Uuid:  chuuid[:],
							Value: chdata,
						})
					}
					logrus.Printf("DataResponse_RetrieveRequest data is extracted %v %v", mime, data)
					stream.Send(&pb.DataRequest{
						Request: &pb.DataRequest_RetrieveResponse{
							RetrieveResponse: &pb.DataRetrieveResponse{
								Data:      dataUUID[:],
								Error:     0,
								Fields:    fields,
								Process:   process[:],
								PublicKey: pubKey[:],
							},
						},
					})

				}
			case *pb.DataResponse_SupplyRequest:
				{
					var pubKey [32]byte
					copy(pubKey[:], u.SupplyRequest.GetPublicKey())
					var dataUUID [16]byte
					copy(dataUUID[:], u.SupplyRequest.GetData())
					var process [16]byte
					copy(process[:], u.SupplyRequest.GetProcess())
					mime := u.SupplyRequest.GetMime()
					err := WriteUserData(&pubKey, &dataUUID, &mime, u.SupplyRequest.GetValue())
					if err != nil {
						logrus.Println("DataResponse_SupplyRequest err", err)
						continue
					}
					logrus.Printf("User %s, uuid %s  written, mime %s", common.B2S(pubKey[:]), common.Bytes2uuid(dataUUID[:]), mime)

				}
			case *pb.DataResponse_DeleteRequest:
				{
					// TODO: delete from db
					msg := &pb.DataRequest{
						Request: &pb.DataRequest_DeleteResponse{
							DeleteResponse: &pb.DataDeleteResponse{
								PublicKey: u.DeleteRequest.GetPublicKey(),
								Data:      u.DeleteRequest.GetData(),
								Error:     0,
							},
						},
					}
					stream.Send(msg)
				}
			}

		}
	}()

	go func() {
		for {
			select {
			case r := <-dataReq:
				var key [48]byte
				copy(key[:], r.Request.RetrieveRequest.PublicKey)
				copy(key[32:], r.Request.RetrieveRequest.Data)
				respChan[key] = r.Response
				stream.Send(&pb.DataRequest{
					Request: r.Request,
				})
			case <-waitChanel:
				return
			}
		}
	}()

	<-waitChanel
	stream.CloseSend()

}
