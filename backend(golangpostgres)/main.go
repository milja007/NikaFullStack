package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil" // Za ioutil.ReadFile, ako još koristiš populateDatabaseFromJSON
	"log"
	"net/http" // NOVI IMPORT za web server
	"os"

	"github.com/jackc/pgx/v5" // Importiramo pgx za pgx.ErrNoRows
	"github.com/jackc/pgx/v5/pgxpool"
)

// Strukture za čitanje data.json (ako se još koristi populateDatabaseFromJSON)
type QuizData struct {
	Kategories []string   `json:"Kategories"` // Lista imena kategorija
	QuizItems  []QuizItem `json:"quizItems"`
}

type QuizItem struct {
	Text       string `json:"text"`
	Score      int    `json:"score"`
	Removable  bool   `json:"removable"`
	Kategorija string `json:"kategorija"` // Ime kategorije kako je u JSON-u
}

// Strukture za API odgovor (što šaljemo frontendu)
type QuizItemAPI struct {
	Text       string `json:"text"`
	Score      int    `json:"score"`
	Removable  bool   `json:"removable"`
	Kategorija string `json:"kategorija"` // Ime kategorije
}

type QuizDataAPIResponse struct {
	Kategories []string      `json:"Kategories"` // Lista imena kategorija
	QuizItems  []QuizItemAPI `json:"quizItems"`
}

// Funkcija za dohvaćanje podataka iz baze za API
func fetchQuizDataFromDB(ctx context.Context, dbpool *pgxpool.Pool) (QuizDataAPIResponse, error) {
	var response QuizDataAPIResponse
	categoryMap := make(map[int]string) // Za mapiranje ID kategorije -> Ime kategorije

	// 1. Dohvati sve kategorije
	rowsCategories, err := dbpool.Query(ctx, "SELECT id, name FROM categories ORDER BY id")
	if err != nil {
		return response, fmt.Errorf("greška pri dohvaćanju kategorija: %w", err)
	}
	defer rowsCategories.Close()

	for rowsCategories.Next() {
		var catID int
		var catName string
		if err := rowsCategories.Scan(&catID, &catName); err != nil {
			return response, fmt.Errorf("greška pri skeniranju kategorije: %w", err)
		}
		response.Kategories = append(response.Kategories, catName)
		categoryMap[catID] = catName
	}
	if err := rowsCategories.Err(); err != nil {
		// Provjeri je li greška pgx.ErrNoRows, što je u redu ako nema kategorija
		if err == pgx.ErrNoRows {
			// Nema kategorija, vrati prazan response.Kategories
		} else {
			return response, fmt.Errorf("greška nakon iteracije kroz kategorije: %w", err)
		}
	}

	// 2. Dohvati sva pitanja
	rowsItems, err := dbpool.Query(ctx, "SELECT text, score, removable, category_id FROM quiz_items ORDER BY category_id, id")
	if err != nil {
		return response, fmt.Errorf("greška pri dohvaćanju pitanja: %w", err)
	}
	defer rowsItems.Close()

	for rowsItems.Next() {
		var item QuizItemAPI
		var categoryID int
		if err := rowsItems.Scan(&item.Text, &item.Score, &item.Removable, &categoryID); err != nil {
			return response, fmt.Errorf("greška pri skeniranju pitanja: %w", err)
		}

		catName, ok := categoryMap[categoryID]
		if !ok {
			log.Printf("Upozorenje: Nije pronađeno ime za category_id %d za pitanje '%s'", categoryID, item.Text)
			item.Kategorija = "Nepoznata kategorija" // Fallback
		} else {
			item.Kategorija = catName
		}
		response.QuizItems = append(response.QuizItems, item)
	}
	if err := rowsItems.Err(); err != nil {
		// Provjeri je li greška pgx.ErrNoRows, što je u redu ako nema pitanja
		if err == pgx.ErrNoRows {
			// Nema pitanja, vrati prazan response.QuizItems
		} else {
			return response, fmt.Errorf("greška nakon iteracije kroz pitanja: %w", err)
		}
	}
	return response, nil
}

// Handler funkcija za /api/quiz-data
func quizDataHandler(w http.ResponseWriter, r *http.Request, dbpool *pgxpool.Pool) {
	// Omogući CORS (Cross-Origin Resource Sharing) - važno za lokalni razvoj
	// kada frontend i backend rade na različitim portovima.
	// Za produkciju, ovo bi trebalo biti konfigurabilnije.
	w.Header().Set("Access-Control-Allow-Origin", "*") // Dopušta zahtjeve s bilo koje domene
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Ako je OPTIONS zahtjev (preflight request za CORS), samo vrati OK
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Metoda nije dopuštena", http.StatusMethodNotAllowed)
		return
	}

	quizData, err := fetchQuizDataFromDB(context.Background(), dbpool)
	if err != nil {
		log.Printf("Greška pri dohvaćanju podataka iz baze: %v", err)
		http.Error(w, "Interna greška servera", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(quizData); err != nil {
		log.Printf("Greška pri kodiranju JSON odgovora: %v", err)
		// Ne možemo poslati http.Error ako su headeri već poslani,
		// ali logiranje je i dalje važno.
	}
}

// Funkcija za brisanje svih podataka iz tablica (opcionalno) - ostaje ista
func clearTables(ctx context.Context, dbpool *pgxpool.Pool) error {
	if _, err := dbpool.Exec(ctx, "DELETE FROM quiz_items"); err != nil {
		return fmt.Errorf("greška pri brisanju tablice quiz_items: %w", err)
	}
	if _, err := dbpool.Exec(ctx, "DELETE FROM categories"); err != nil {
		return fmt.Errorf("greška pri brisanju tablice categories: %w", err)
	}
	fmt.Println("Tablice 'quiz_items' i 'categories' su obrisane (ako su postojale).")
	// Za potpuni reset auto-increment ID-jeva, umjesto DELETE može se koristiti:
	// _, err := dbpool.Exec(ctx, "TRUNCATE TABLE quiz_items, categories RESTART IDENTITY CASCADE")
	// if err != nil {
	//    return fmt.Errorf("failed to truncate tables: %w", err)
	// }
	return nil
}

// Funkcija za popunjavanje baze iz JSON-a - ostaje ista
func populateDatabaseFromJSON(ctx context.Context, dbpool *pgxpool.Pool, jsonFilePath string) error {
	byteValue, err := ioutil.ReadFile(jsonFilePath)
	if err != nil {
		return fmt.Errorf("greška pri čitanju datoteke %s: %w", jsonFilePath, err)
	}

	var quizData QuizData
	if err := json.Unmarshal(byteValue, &quizData); err != nil {
		return fmt.Errorf("greška pri parsiranju JSON podataka: %w", err)
	}
	fmt.Println("JSON podaci uspješno parsirani (iz populateDatabaseFromJSON).")

	categoryNameToID := make(map[string]int)
	fmt.Println("Popunjavanje tablice 'categories' (iz populateDatabaseFromJSON)...")

	for _, categoryName := range quizData.Kategories {
		var categoryID int
		err := dbpool.QueryRow(ctx, "SELECT id FROM categories WHERE name = $1", categoryName).Scan(&categoryID)
		if err != nil {
			if err == pgx.ErrNoRows {
				insertSQL := "INSERT INTO categories (name) VALUES ($1) RETURNING id"
				errInsert := dbpool.QueryRow(ctx, insertSQL, categoryName).Scan(&categoryID)
				if errInsert != nil {
					return fmt.Errorf("greška pri unosu kategorije '%s': %w", categoryName, errInsert)
				}
				fmt.Printf("Unesena kategorija: '%s' s ID-om: %d (iz populateDatabaseFromJSON)\n", categoryName, categoryID)
			} else {
				return fmt.Errorf("greška pri provjeri kategorije '%s': %w", categoryName, err)
			}
		} else {
			fmt.Printf("Kategorija '%s' već postoji s ID-om: %d (iz populateDatabaseFromJSON)\n", categoryName, categoryID)
		}
		categoryNameToID[categoryName] = categoryID
	}
	fmt.Println("Tablica 'categories' popunjena (iz populateDatabaseFromJSON).")

	fmt.Println("Popunjavanje tablice 'quiz_items' (iz populateDatabaseFromJSON)...")
	for i, item := range quizData.QuizItems {
		categoryID, ok := categoryNameToID[item.Kategorija]
		if !ok {
			return fmt.Errorf("kategorija '%s' za pitanje '%s' (index %d) nije pronađena u mapi. Provjerite data.json.", item.Kategorija, item.Text, i)
		}

		// Provjera da li pitanje već postoji da se izbjegnu duplikati ako se populateDatabaseFromJSON poziva više puta
		var exists bool
		checkSQL := "SELECT EXISTS(SELECT 1 FROM quiz_items WHERE text = $1 AND category_id = $2)"
		err = dbpool.QueryRow(ctx, checkSQL, item.Text, categoryID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("greška pri provjeri postojanja pitanja '%s': %w", item.Text, err)
		}

		if !exists {
			insertSQL := "INSERT INTO quiz_items (text, score, removable, category_id) VALUES ($1, $2, $3, $4)"
			_, err := dbpool.Exec(ctx, insertSQL, item.Text, item.Score, item.Removable, categoryID)
			if err != nil {
				return fmt.Errorf("greška pri unosu pitanja '%s' (index %d): %w", item.Text, i, err)
			}
			// fmt.Printf("Uneseno pitanje: '%s' (iz populateDatabaseFromJSON)\n", item.Text) // Može biti previše ispisa
		} else {
			// fmt.Printf("Pitanje '%s' već postoji, preskačem unos (iz populateDatabaseFromJSON).\n", item.Text) // Može biti previše ispisa
		}
	}
	fmt.Println("Tablica 'quiz_items' popunjena (iz populateDatabaseFromJSON).")
	return nil
}

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgresql://checkbox_app_user:12345678@localhost:5432/checkbox_app_db" // Tvoja lozinka
		log.Println("UPOZORENJE: DATABASE_URL nije postavljen. Koristim zadanu vrijednost.")
	}

	dbpool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("Nije moguće kreirati connection pool: %v\n", err)
	}
	defer dbpool.Close()

	if err := dbpool.Ping(context.Background()); err != nil {
		log.Fatalf("Nije moguće spojiti se na bazu: %v\n", err)
	}
	fmt.Println("Uspješno spojeni na PostgreSQL!")

	// OPCIONALNO: Popunjavanje baze ako je potrebno.
	// Za produkcijski server, ovo se obično ne radi pri svakom pokretanju.
	// Možeš ostaviti zakomentirano ako su podaci već u bazi.
	// Ako želiš da se baza uvijek iznova popuni (nakon brisanja), otkomentiraj i clearTables.
	/*
		if err := clearTables(context.Background(), dbpool); err != nil {
			log.Fatalf("Greška prilikom brisanja tablica: %v\n", err)
		}
	*/
	/*
		// Pozovi populateDatabaseFromJSON samo ako želiš da se podaci unose/ažuriraju pri svakom startu servera.
		// Ovo će pokušati unijeti podatke, preskačući duplikate ako već postoje.
		if err := populateDatabaseFromJSON(context.Background(), dbpool, "data.json"); err != nil {
			log.Fatalf("Greška prilikom popunjavanja baze podataka: %v\n", err)
		}
		fmt.Println("Baza podataka inicijalizirana/ažurirana podacima iz data.json.");
	*/


	mux := http.NewServeMux()

	mux.HandleFunc("/api/quiz-data", func(w http.ResponseWriter, r *http.Request) {
		quizDataHandler(w, r, dbpool)
	})

	port := "8080"
	fmt.Printf("Pokrećem server na portu %s...\n", port)
	fmt.Printf("API za kviz je dostupan na: http://localhost:%s/api/quiz-data\n", port)

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Greška pri pokretanju servera: %v\n", err)
	}
}
