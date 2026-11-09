package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// NominatimMinInterval is OSM's usage policy for the public endpoint: at most
// one request per second. Callers backfilling in bulk must respect it.
const NominatimMinInterval = time.Second

// nominatimUserAgent identifies this tool, as OSM's policy requires.
const nominatimUserAgent = "running-cli/1.0 (personal training log)"

type nominatimResponse struct {
	Address struct {
		City        string `json:"city"`
		Town        string `json:"town"`
		Village     string `json:"village"`
		Hamlet      string `json:"hamlet"`
		Suburb      string `json:"suburb"`
		Municipality string `json:"municipality"`
		County      string `json:"county"`
	} `json:"address"`
}

// ReverseGeocode turns coordinates into a locality name. It returns just the
// town/city — matching the bare names Garmin supplies ("Belmont", "Natick") so
// backfilled rows read the same as existing ones.
func ReverseGeocode(lat, lon float64) (string, error) {
	url := fmt.Sprintf(
		"https://nominatim.openstreetmap.org/reverse?format=jsonv2&lat=%.5f&lon=%.5f&zoom=12&addressdetails=1",
		lat, lon,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", nominatimUserAgent)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("nominatim responded %d", resp.StatusCode)
	}

	var payload nominatimResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}

	a := payload.Address
	for _, name := range []string{a.City, a.Town, a.Village, a.Municipality, a.Suburb, a.Hamlet, a.County} {
		if name != "" {
			return name, nil
		}
	}

	return "", nil
}
