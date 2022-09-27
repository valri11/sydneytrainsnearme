package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	jsoniter "github.com/json-iterator/go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/proto"

	"github.com/valri11/sydneytrainsnearme/config"
	gtfs "github.com/valri11/sydneytrainsnearme/protobuf_gen"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// getTrainPosCmd represents the getTrainPos command
var getTrainPosCmd = &cobra.Command{
	Use:   "get-train-pos",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: getTrainPosCommand,
}

func init() {
	rootCmd.AddCommand(getTrainPosCmd)
}

func getTrainPosCommand(cmd *cobra.Command, args []string) {
	var cfg config.Configuration
	err := viper.Unmarshal(&cfg)
	if err != nil {
		log.Fatalf("error unmarshal config: %v", err)
		return
	}

	if cfg.NswPublicTransportApi.ApiKey == "" {
		log.Fatalf("no API key set")
		return
	}

	reqUrl := "https://api.transport.nsw.gov.au/v2/gtfs/vehiclepos/sydneytrains"

	client := http.Client{}

	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		log.Fatalf("ERR: %v", err)
		return
	}

	req.Header = http.Header{
		"Content-Type":  {"application/x-google-protobuf"},
		"Authorization": {fmt.Sprintf("apikey %s", cfg.NswPublicTransportApi.ApiKey)},
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("ERR: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("ERR: %v", err)
		return
	}

	var vp gtfs.FeedMessage

	err = proto.Unmarshal(body, &vp)
	if err != nil {
		log.Fatalln("Error reading file:", err)
	}

	//ts := time.Unix(int64(*vp.Header.Timestamp), 0)
	//fmt.Printf("Timestamp: %v\n", ts)

	//	for _, ent := range vp.Entity {
	//		fmt.Printf("%v\n", ent.Vehicle.Vehicle)
	//		fmt.Printf("Position: %v\n", ent.Vehicle.Position)
	//	}

	fc, err := vehiclePositionToFeatureCollection(vp.Entity)
	if err != nil {
		log.Fatalf("ERR: %v", err)
		return
	}

	out, err := json.Marshal(fc)
	if err != nil {
		log.Fatalf("ERR: %v", err)
		return
	}
	fmt.Printf("%s\n", string(out))
}

func vehiclePositionToFeatureCollection(vpe []*gtfs.FeedEntity) (*FeatureCollection, error) {
	fc := FeatureCollection{Type: "FeatureCollection"}

	feat := make([]Feature, 0)
	for _, ent := range vpe {
		if ent.Vehicle.Position == nil {
			continue
		}
		ft := Feature{Type: "Feature"}

		geom := Geometry{Type: "Point"}
		geom.Coordinates = make([]float64, 2)
		geom.Coordinates[0] = float64(*ent.Vehicle.Position.Longitude)
		geom.Coordinates[1] = float64(*ent.Vehicle.Position.Latitude)

		ft.Geometry = geom

		ft.Properties = make(Properties)
		ft.Properties["id"] = ent.Vehicle.Vehicle.Id
		if ent.Vehicle.Vehicle.Label != nil {
			ft.Properties["label"] = *ent.Vehicle.Vehicle.Label
		}

		feat = append(feat, ft)
	}
	fc.Features = feat

	return &fc, nil
}
