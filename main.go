package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"

)

//PrivacyPolicy represents the data structure for the privacy policy

type PrivacyPolicy struct {
	ID                 int
	CompanyName        string
	Email              string
	Website            string
	Country            string
	RegistrationNumber string
	Address            string
}

// TemplateData represents the data structure for template rendering

type TemplateData struct {
	PrivacyPolicy PrivacyPolicy
	Template      string
}

// Database connection parameters
const (
	dbDriver   = "mysql"
	dbUser     = "root"
	dbPassword = "password"
	dbName     = "privacy_policy_db"
)

// Create the "privacy_policies" table if it doesn't exist

const createTableQuery = `
CREATE TABLE IF NOT EXISTS privacy_policies (
	id INT AUTO_INCREMENT PRIMARY KEY,
	company_name VARCHAR(255) NOT NULL,
	email VARCHAR(255) NOT NULL,
	website VARCHAR(255) NOT NULL,
	country VARCHAR(255) NOT NULL,
	registration_number VARCHAR(255) NOT NULL,
	address VARCHAR(255) NOT NULL
);
`

// Insert a privacy policy into the database
const insertQuery = `
INSERT INTO privacy_policies (company_name, email, website, country, registration_number, address)
VALUES (?, ?, ?, ?, ?, ?);
`

const selectQuery = `
SELECT * FROM privacy_policies WHERE id = ?;
`

//Privacy Policy templates

const (
NDPRFileName = "ndpr.html"
GDPRFileName = "gdpr.html"
CCPAFileName = "ccpa.html"
)

func main() {
	//open a database connection
	db, err := sql.Open(dbDriver, fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/%s", dbUser, dbPassword, dbName))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	//Creat the "privacy_policies" table if it doesn't exist
	_, err = db.Exec(createTableQuery)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize Gin

	router := gin.Default()

	//Set up a route to handle the web interface
	router.LoadHTMLGlob("templates/*")
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	router.POST("/generate", func(c *gin.Context) {
		//Parse form data
		var data PrivacyPolicy
		if err := c.ShouldBind(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Insert the privacy policy into the database
		result, err := db.Exec(insertQuery, data.CompanyName, data.Email, data.Website, data.Country, data.RegistrationNumber, data.Address)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Retrive the inserted privacy policy by ID
		lastInsertID, _ := result.LastInsertId()
		retrievedPolicy := PrivacyPolicy{}
		err = db.QueryRow(selectQuery, lastInsertID).Scan(
			&retrievedPolicy.ID,
			&retrievedPolicy.CompanyName,
			&retrievedPolicy.Email,
			&retrievedPolicy.Website,
			&retrievedPolicy.Country,
			&retrievedPolicy.RegistrationNumber,
			&retrievedPolicy.Address,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}


		selectedPolicyType := c.PostForm("PolicyType")

		renderedPolicies := map[string]string{
			selectedPolicyType: renderTemplate(loadTemplate(selectedPolicyType, selectedPolicyType+".html"), retrievedPolicy),
		}
		


		c.HTML(http.StatusOK, "generated_policy.html", gin.H{
			"SelectedPolicy": selectedPolicyType,
			"PolicyContent": renderedPolicies[selectedPolicyType],
		})


	})

	//Run the web server
	router.Run(":8080")
}

func renderTemplate(tmpl *template.Template, data PrivacyPolicy) string {
	var result bytes.Buffer
	templateData := TemplateData{
		PrivacyPolicy: data,
		Template:      "Generated Privacy Policy:",
	}
	
	err := tmpl.ExecuteTemplate(&result, "base", templateData)
	if err != nil {
		log.Fatal(err)
	}
	resultString := result.String()
	log.Printf("Generated HTML: %v", resultString)
	return resultString
}


func loadTemplate(name, fileName string) *template.Template {
	tmpl, err := template.New(name).ParseFiles("templates/base.html", "templates/"+fileName)
	if err != nil {
		log.Fatal(err)
	}
	return tmpl
}
