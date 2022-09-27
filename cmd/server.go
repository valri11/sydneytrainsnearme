/*
Copyright © 2022 Val Gridnev valer.gr@gmail.com
*/
package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/proto"

	"github.com/valri11/sydneytrainsnearme/config"
	gtfs "github.com/valri11/sydneytrainsnearme/protobuf_gen"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: doWebServer,
}

type dataItem struct {
	ts   time.Time
	data []byte
}

type transportHandler struct {
	cfg  config.Configuration
	data dataItem
}

func init() {
	rootCmd.AddCommand(serverCmd)

	serverCmd.Flags().BoolP("dev-mode", "", false, "development mode (http on loclahost)")
	serverCmd.Flags().String("tls-cert", "", "TLS certificate file")
	serverCmd.Flags().String("tls-cert-key", "", "TLS certificate key file")
	serverCmd.Flags().Int("port", 8030, "service port to listen")
}

func newTransportHandler() (*transportHandler, error) {
	var cfg config.Configuration
	err := viper.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}

	t := transportHandler{
		cfg: cfg,
	}

	return &t, nil
}

func doWebServer(cmd *cobra.Command, args []string) {
	devMode, err := cmd.Flags().GetBool("dev-mode")
	if err != nil {
		log.Fatalf("ERR: %v", err)
		return
	}

	servicePort, err := cmd.Flags().GetInt("port")
	if err != nil {
		log.Fatalf("ERR: %v", err)
		return
	}

	tlsCertFile, err := cmd.Flags().GetString("tls-cert")
	if err != nil {
		log.Fatalf("ERR: %v", err)
		return
	}

	tlsCertKeyFile, err := cmd.Flags().GetString("tls-cert-key")
	if err != nil {
		log.Fatalf("ERR: %v", err)
		return
	}

	if !devMode {
		if tlsCertFile == "" || tlsCertKeyFile == "" {
			fmt.Println("must provide TLS key and certificate")
			return
		}
	}

	t, err := newTransportHandler()
	if err != nil {
		log.Fatalf("ERR: %v", err)
		return
	}

	r := mux.NewRouter()
	r.HandleFunc("/sydneytrainsnearme.geojson", t.sydneyTrainsNearbyHandler)
	r.HandleFunc("/sydneytrainsnearme.sse", t.sydneyTrainsNearbyPushHandler)
	r.HandleFunc("/sydneytrainsnearme.ws", t.sydneyTrainsNearbyWebSockHandler)

	// Where ORIGIN_ALLOWED is like `scheme://dns[:port]`, or `*` (insecure)
	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With", "content-type", "username", "password", "Referer"})
	originsOk := handlers.AllowedOrigins([]string{"*"})
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS"})

	// start server listen with error handling
	mux := handlers.CORS(originsOk, headersOk, methodsOk)(r)
	srv := &http.Server{
		Addr:        fmt.Sprintf("0.0.0.0:%d", servicePort),
		Handler:     mux,
		IdleTimeout: time.Minute,
		//ReadTimeout:  20 * time.Second,
		//WriteTimeout: 30 * time.Second,
	}

	if devMode {
		err = srv.ListenAndServe()
	} else {
		err = srv.ListenAndServeTLS(tlsCertFile, tlsCertKeyFile)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func (h *transportHandler) sydneyTrainsNearbyHandler(w http.ResponseWriter, r *http.Request) {

	out, err := h.reqVehiclePosition()
	if err != nil {
		log.Fatalf("ERR: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

func (h *transportHandler) sydneyTrainsNearbyPushHandler(w http.ResponseWriter, r *http.Request) {

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Internal error", 500)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			out, err := h.reqVehiclePosition()
			if err != nil {
				log.Printf("ERR: %v", err)
				continue
			}
			fmt.Fprintf(w, "data: %v\n\n", string(out))
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (h *transportHandler) sydneyTrainsNearbyWebSockHandler(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	// upgrade this connection to a WebSocket connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}

	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			out, err := h.reqVehiclePosition()
			if err != nil {
				log.Printf("ERR: %v", err)
				continue
			}
			err = ws.WriteMessage(websocket.TextMessage, out)
			if err != nil {
				log.Printf("ERR: %v", err)
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (h *transportHandler) reqVehiclePosition() ([]byte, error) {

	nowTs := time.Now()
	if nowTs.Sub(h.data.ts) <= 1*time.Second {
		log.Printf("cache time: %v", nowTs.Sub(h.data.ts))
		return h.data.data, nil
	}

	reqUrl := "https://api.transport.nsw.gov.au/v2/gtfs/vehiclepos/sydneytrains"

	client := http.Client{}

	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		log.Printf("ERR: %v", err)
		return nil, err
	}

	req.Header = http.Header{
		"Content-Type":  {"application/x-google-protobuf"},
		"Authorization": {fmt.Sprintf("apikey %s", h.cfg.NswPublicTransportApi.ApiKey)},
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("ERR: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERR: %v", err)
		return nil, err
	}

	var vp gtfs.FeedMessage

	err = proto.Unmarshal(body, &vp)
	if err != nil {
		log.Printf("Error unmarshal: %v", err)
		return nil, err
	}

	fc, err := vehiclePositionToFeatureCollection(vp.Entity)
	if err != nil {
		log.Printf("ERR: %v", err)
		return nil, err
	}

	out, err := json.Marshal(fc)
	if err != nil {
		log.Printf("ERR: %v", err)
		return nil, err
	}

	//return out, nil

	h.data.ts = time.Now()
	h.data.data = out

	return h.data.data, nil
}
