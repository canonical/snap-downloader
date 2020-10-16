package store

import (
	"encoding/json"
	"fmt"
	"github.com/slimjim777/snap-downloader/service/datastore"
	"log"
)

// Service is the interface for the store
type Service interface {
	LoginUser(email, password, otp, storeID, series string) error
	SnapInfo(name string) (*ResponseSnapInfo, error)
	Macaroon() (map[string]string, error)
}

// SnapStore interacts with a brand store
type SnapStore struct {
	Datastore datastore.Datastore
	headers   map[string]string
}

// NewStore creates a new store client
func NewStore(ds datastore.Datastore) *SnapStore {
	// check if we have cached headers (with the store macaroon)
	cfg, err := ds.SettingsGet("store", "headers")
	if err != nil {
		return &SnapStore{
			Datastore: ds,
		}
	}
	headers, err := readHeaders([]byte(cfg.Data))
	if err != nil {
		return &SnapStore{
			Datastore: ds,
		}
	}

	// use the cached headers, so no login needed
	return &SnapStore{
		Datastore: ds,
		headers:   headers,
	}
}

// LoginUser login to the store and request needed ACLs
func (sto *SnapStore) LoginUser(email, password, otp, storeID, series string) error {
	macaroon, discharge, err := LoginUser(email, password, otp, []string{"package_access"})
	if err != nil {
		return err
	}

	authHeader, err := AuthorizationHeader(macaroon, discharge)
	if err != nil {
		return err
	}

	// set the headers to access the brand store
	sto.headers = map[string]string{
		"Snap-Device-Store":   storeID,
		"Snap-Device-Series":  series,
		"Snap-Device-Channel": "stable",
		"Authorization":       authHeader,
		"Content-Type":        "application/json",
		"Accept":              "application/json",
	}

	// cache the headers in the database
	data, err := json.Marshal(sto.headers)
	if err != nil {
		return err
	}
	_, err = sto.Datastore.SettingsPut("store", "headers", string(data))
	return err
}

// Macaroon returns the stored macaroon from the data store
func (sto *SnapStore) Macaroon() (map[string]string, error) {
	cfg, err := sto.Datastore.SettingsGet("store", "headers")
	if err != nil {
		return nil, err
	}

	headers, err := readHeaders([]byte(cfg.Data))
	if err != nil {
		return nil, err
	}

	// remove the actual macaroon from the response
	delete(headers, "Authorization")
	delete(headers, "Content-Type")
	delete(headers, "Accept")

	headers["Created"] = cfg.Created.String()
	headers["Modified"] = cfg.Modified.String()
	return headers, nil
}

// SnapInfo lists the snaps in a brand store
func (sto SnapStore) SnapInfo(name string) (*ResponseSnapInfo, error) {
	u := fmt.Sprintf("%s/snaps/info/%s", apiBaseURL, name)
	resp, err := submitGETRequest(u, sto.headers)
	if err != nil {
		log.Printf("Error fetching snap info: %v", err)
		return nil, err
	}

	var response ResponseSnapInfo

	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}
