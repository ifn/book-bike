package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
)

const AutoRuVendorsUrl = "http://moto.auto.ru/motorcycle/"

func queryToAutoRuQuery(query string) string {
	model := query

	model_ids := map[string]string{
		"VFR800": "7889",
	}

	model_param := "m[]=" + model_ids[model]

	model_paths := map[string]string{
		"VFR800": "used/honda/vfr/",
	}

	return AutoRuVendorsUrl + model_paths[model] + "?" + model_param
}

func getAutoRuOffers(query string) <-chan interface{} {
	links := make(chan interface{})

	go func() {
		auto_ru_query := queryToAutoRuQuery(query)

		resp, err := http.Get(auto_ru_query)
		if err != nil {
			links <- err
			close(links)
			return
		}
		defer resp.Body.Close()

		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			links <- err
			close(links)
			return
		}

		//parse, fetch links
		log.Println(string(respBody))

		links <- "http://moto.auto.ru/motorcycle/used/sale/1826802-02dbed.html"
		close(links)
	}()

	return links
}

func getAvitoOffers(query string) <-chan interface{} {
	links := make(chan interface{})
	go func() {
		links <- "https://www.avito.ru/moskva/mototsikly_i_mototehnika/honda_cbr_1000_rr_fireblade_632522473"
		close(links)
	}()
	return links
}

func (self *BikeOffersResponse) getOffers(query string) (offers []string, err error) {
	auto := getAutoRuOffers(query)
	var avito chan interface{}

	for auto != nil || avito != nil {
		select {
		case msg, ok := <-auto:
			if !ok {
				auto = nil
				break
			}

			switch msg := msg.(type) {
			case error:
				err = msg
				auto = nil
				break
			case string:
				offers = append(offers, msg)
			}
		}
	}

	return
}

//

type state struct {
	db *sql.DB
}

//

type Error struct {
	Err string `json:"error"`
}

type BikeOffersRequest struct {
	Model string `json:"model"`
}

type BikeOffersResponse struct {
	st *state

	*BikeOffersRequest
	Offers []string `json:"offers"`
}

//

func (self *BikeOffersResponse) SetOffers() error {
	model := self.Model

	offers, err := self.getOffers(model)
	if err != nil {
		return err
	}

	self.Offers = offers
	return nil
}

func getBikeOffers(st *state) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		decoder := json.NewDecoder(r.Body)
		encoder := json.NewEncoder(w)

		resp := &BikeOffersResponse{st: st}

		err := decoder.Decode(resp)
		if err != nil {
			log.Println(err)
			encoder.Encode(Error{err.Error()})
			return
		}

		err = resp.SetOffers()
		if err != nil {
			log.Println(err)
			encoder.Encode(Error{err.Error()})
			return
		}

		err = encoder.Encode(resp)
		if err != nil {
			log.Println(err)
			encoder.Encode(Error{err.Error()})
			return
		}
	}
}

func startBBSrv() {
	st := &state{}

	r := mux.NewRouter()
	r.HandleFunc("/getBikeOffers", getBikeOffers(st)).Methods("POST")
	http.Handle("/", r)

	n := negroni.Classic()
	n.UseHandler(r)
	n.Run(":" + os.Getenv("PORT"))
}

func main() {
	startBBSrv()
}

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}
