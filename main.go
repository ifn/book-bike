package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"runtime"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
)

func getAutoOffers(model string) <-chan string {
	links := make(chan string)
	go func() {
		links <- "http://moto.auto.ru/motorcycle/used/sale/1826802-02dbed.html"
		close(links)
	}()
	return links
}

func getAvitoOffers(model string) <-chan string {
	links := make(chan string)
	go func() {
		links <- "https://www.avito.ru/moskva/mototsikly_i_mototehnika/honda_cbr_1000_rr_fireblade_632522473"
		close(links)
	}()
	return links
}

func getOffers(model string) (offers []string) {
	auto := getAutoOffers(model)
	avito := getAvitoOffers(model)

	for auto != nil || avito != nil {
		select {
		case link, ok := <-auto:
			if !ok {
				auto = nil
				break
			}
			offers = append(offers, link)
		case link, ok := <-avito:
			if !ok {
				avito = nil
				break
			}
			offers = append(offers, link)
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

func (self *BikeOffersResponse) getDbOffers(model string) (offers []string, err error) {
	db := self.st.db

	rows, err := db.Query("SELECT link FROM bikes WHERE model = ?", model)
	if err != nil {
		return
	}
	for rows.Next() {
		var link string
		if err = rows.Scan(&link); err != nil {
			return
		}
		offers = append(offers, link)
	}
	if err = rows.Err(); err != nil {
		return
	}
	return
}

func (self *BikeOffersResponse) SetOffers() error {
	model := self.Model

	offers, err := self.getDbOffers(model)
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
	db, err := sql.Open("mysql", "bb:123456@tcp(dbserver:3306)/bikes?charset=utf8&parseTime=True&loc=Local")
	if err != nil {
		log.Fatal(err)
	}

	st := &state{db: db}

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
