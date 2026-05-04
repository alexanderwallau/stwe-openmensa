package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// canteenSlugs maps canteen slugs (used in URLs and as keys) to themselves.
// The slug is the identifier used by stw-edu.de.
var canteenSlugs = map[string]string{
	"mensa-campus-duisburg":  "mensa-campus-duisburg",
	"mensa-campus-essen":     "mensa-campus-essen",
	"mensa-campus-bottrop":   "mensa-campus-bottrop",
	"mensa-campus-folkwang":  "mensa-campus-folkwang",
	"mensa-campus-muelheim":  "mensa-campus-muelheim",
	"bistro-campus-zollverein": "bistro-campus-zollverein",
}

// CanteenInfo holds static metadata for a canteen.
type CanteenInfo struct {
	Name      string
	Address   string
	City      string
	Phone     string
	Latitude  float64
	Longitude float64
	// Hours contains opening hours for Mon–Sun (index 0=Mon … 6=Sun).
	// An empty string means closed that day.
	Hours [7]string
}

// canteenInfoMap holds the static metadata for each canteen.
var canteenInfoMap = map[string]CanteenInfo{
	"mensa-campus-duisburg": {
		Name:      "Mensa Campus Duisburg",
		Address:   "Lotharstraße 23-25, 47057 Duisburg",
		City:      "Duisburg",
		Phone:     "+49 203-379-1907",
		Latitude:  51.4330,
		Longitude: 6.7669,
		Hours:     [7]string{"11:15-14:30", "11:15-14:30", "11:15-14:30", "11:15-14:30", "11:15-14:15", "", ""},
	},
	"mensa-campus-essen": {
		Name:      "Mensa Campus Essen",
		Address:   "Universitätsstraße 2, 45141 Essen",
		City:      "Essen",
		Phone:     "+49 203-379-1907",
		Latitude:  51.4641,
		Longitude: 7.0092,
		Hours:     [7]string{"11:15-14:30", "11:15-14:30", "11:15-14:30", "11:15-14:30", "11:15-14:15", "", ""},
	},
	"mensa-campus-bottrop": {
		Name:      "Mensa Campus Bottrop",
		Address:   "Lützowstraße 5, 46236 Bottrop",
		City:      "Bottrop",
		Phone:     "+49 203-379-1907",
		Latitude:  51.5220,
		Longitude: 6.9286,
		Hours:     [7]string{"11:15-14:30", "11:15-14:30", "11:15-14:30", "11:15-14:30", "11:15-14:30", "", ""},
	},
	"mensa-campus-folkwang": {
		Name:      "Mensa Campus Folkwang",
		Address:   "Klemensborn 39, 45239 Essen",
		City:      "Essen",
		Phone:     "+49 203-379-1907",
		Latitude:  51.3992,
		Longitude: 6.9907,
		Hours:     [7]string{"11:30-14:30", "11:30-14:30", "11:30-14:30", "11:30-14:30", "11:30-14:30", "", ""},
	},
	"mensa-campus-muelheim": {
		Name:      "Mensa Campus Mülheim",
		Address:   "Duisburger Straße 100, 45479 Mülheim an der Ruhr",
		City:      "Mülheim an der Ruhr",
		Phone:     "+49 203-379-1907",
		Latitude:  51.4285,
		Longitude: 6.8829,
		Hours:     [7]string{"11:15-14:30", "11:15-14:30", "11:15-14:30", "11:15-14:30", "11:15-14:30", "", ""},
	},
	"bistro-campus-zollverein": {
		Name:      "Bistro Campus Zollverein",
		Address:   "Bullmannaue 11, 45327 Essen",
		City:      "Essen",
		Phone:     "+49 203-379-1907",
		Latitude:  51.4878,
		Longitude: 7.0491,
		Hours:     [7]string{"11:15-14:30", "11:15-14:30", "11:15-14:30", "11:15-14:30", "11:15-14:30", "", ""},
	},
}

const apiURL = "https://www.stw-edu.de/wp-json/az-speisen-grid/v1/items"

// germanDays maps Go's time.Weekday (Sunday=0) to German day names.
var germanDays = [7]string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}

// toGermanDate converts a YYYY-MM-DD date string to the German format
// "Wochentag, DD.MM.YYYY" used by the stw-edu.de API.
func toGermanDate(date string) (string, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", fmt.Errorf("invalid date %q: %w", date, err)
	}
	return fmt.Sprintf("%s, %02d.%02d.%04d",
		germanDays[t.Weekday()], t.Day(), int(t.Month()), t.Year()), nil
}

// Meal holds a single menu item and its metadata.
type Meal struct {
	Title        string
	Allergens    []string // full descriptions, e.g. "i: enthält Sellerie"
	StudentPrice int      // euro cents
	StaffPrice   int      // euro cents (Nicht-Stud., same as GuestPrice)
	GuestPrice   int      // euro cents (Nicht-Stud.)
}

// Category holds a named group of meals.
type Category struct {
	Title string
	Meals []*Meal
}

// apiResponse is the JSON response from the az-speisen-grid REST endpoint.
type apiResponse struct {
	HTML  string `json:"html"`
	Count int    `json:"count"`
}

// FetchMenu downloads and parses the menu for a given canteen and date.
// date must be in YYYY-MM-DD format.
func FetchMenu(canteen, date string) ([]*Category, error) {
	if _, ok := canteenSlugs[canteen]; !ok {
		return nil, fmt.Errorf("unknown canteen %q", canteen)
	}

	germanDate, err := toGermanDate(date)
	if err != nil {
		return nil, err
	}

	params := url.Values{
		"slug":         {canteen},
		"ort":          {canteen},
		"default_date": {germanDate},
	}
	reqURL := apiURL + "?" + params.Encode()

	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var result apiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	return parseMenuHTML(result.HTML)
}

// headingTitleRe matches the text inside elementor heading title elements.
var headingTitleRe = regexp.MustCompile(`elementor-heading-title[^>]*>([^<]+)<`)

// h2Re matches the text inside an h2 elementor heading title.
var h2Re = regexp.MustCompile(`<h2[^>]*elementor-heading-title[^>]*>([^<]+)</h2>`)

// allergenContentRe matches the content of the allergene-content container.
var allergenContentRe = regexp.MustCompile(`(?s)allergene-content[^>]*>(.+?)</div>\s*</div>\s*</div>\s*</div>\s*</div>`)

// priceRe matches euro price strings like "1,86 €" and "10,50 €".
var priceRe = regexp.MustCompile(`\d`)

// parsePrice extracts all digits from a price string and returns euro cents.
// e.g. "1,86 €" → 186
func parsePrice(s string) int {
	n := 0
	for _, d := range priceRe.FindAllString(s, -1) {
		n = n*10 + int(d[0]-'0')
	}
	return n
}

// parseMenuHTML parses the Elementor HTML returned by the az-speisen-grid API.
// Each meal item follows a predictable heading sequence:
//  1. Category name (orange heading)
//  2. Meal title (h2)
//  3. Optional allergen codes "(x,y,z)"
//  4. "Stud. " label, then student price
//  5. "Nicht-Stud. " label, then non-student price
func parseMenuHTML(htmlStr string) ([]*Category, error) {
	if htmlStr == "" {
		return nil, nil
	}

	// Split HTML at speiseplan loop-item boundaries.
	// Each item starts with a div whose class includes "post-NNN speiseplan".
	itemBoundaryRe := regexp.MustCompile(`(?s)<div[^>]+\bpost-\d+\s+speiseplan\b`)
	boundaries := itemBoundaryRe.FindAllStringIndex(htmlStr, -1)
	if len(boundaries) == 0 {
		return nil, nil
	}

	var cats []*Category
	catOrder := []string{}
	catMap := make(map[string]*Category)

	for i, b := range boundaries {
		var itemHTML string
		if i+1 < len(boundaries) {
			itemHTML = htmlStr[b[0]:boundaries[i+1][0]]
		} else {
			itemHTML = htmlStr[b[0]:]
		}

		if !strings.Contains(itemHTML, "speisen-item") {
			continue
		}

		// Extract all heading title texts in document order.
		allMatches := headingTitleRe.FindAllStringSubmatch(itemHTML, -1)
		texts := make([]string, 0, len(allMatches))
		for _, m := range allMatches {
			t := html.UnescapeString(strings.TrimSpace(m[1]))
			if t != "" {
				texts = append(texts, t)
			}
		}
		if len(texts) < 4 {
			// Need at least: category, title, price-label, price
			continue
		}

		// texts[0] = category name
		catName := texts[0]

		// texts[1] = meal title (also present as h2)
		h2Match := h2Re.FindStringSubmatch(itemHTML)
		if h2Match == nil {
			continue
		}
		mealTitle := html.UnescapeString(strings.TrimSpace(h2Match[1]))

		meal := &Meal{Title: mealTitle}

		// texts[2] is either allergen codes "(x,y,z)" or "Stud. " directly.
		idx := 2
		if strings.HasPrefix(texts[idx], "(") && strings.HasSuffix(texts[idx], ")") {
			// Skip the allergen code summary; full details come from allergene-content.
			idx++
		}

		// Parse price labels and values.
		for i := idx; i+1 < len(texts); i++ {
			label := strings.TrimSpace(texts[i])
			val := texts[i+1]
			switch label {
			case "Stud.":
				meal.StudentPrice = parsePrice(val)
				i++ // consume the value
			case "Nicht-Stud.":
				price := parsePrice(val)
				meal.StaffPrice = price
				meal.GuestPrice = price
				i++
			case "Nähr\u00adwerte", "Nährwerte":
				// Reached the nutritional info section; stop price parsing.
				goto doneWithPrices
			}
		}
	doneWithPrices:

		// Extract detailed allergen descriptions from the hidden allergene-content div.
		acMatch := allergenContentRe.FindStringSubmatch(itemHTML)
		if acMatch != nil {
			acTitles := headingTitleRe.FindAllStringSubmatch(acMatch[1], -1)
			for _, t := range acTitles {
				desc := html.UnescapeString(strings.TrimSpace(t[1]))
				if desc != "" && desc != "Zusatzstoffe und Allergene" {
					meal.Allergens = append(meal.Allergens, desc)
				}
			}
		}

		// Add meal to its category, preserving insertion order.
		if _, ok := catMap[catName]; !ok {
			cat := &Category{Title: catName}
			catMap[catName] = cat
			catOrder = append(catOrder, catName)
		}
		catMap[catName].Meals = append(catMap[catName].Meals, meal)
	}

	// Return categories in the order they first appeared.
	for _, name := range catOrder {
		cats = append(cats, catMap[name])
	}
	return cats, nil
}
