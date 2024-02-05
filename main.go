package main

import (
	"bytes"
	"database/sql"
	"html"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"context"
    
	openai "github.com/sashabaranov/go-openai"
	"github.com/joho/godotenv"
    "github.com/bcongdon/fn"
	"github.com/gin-gonic/gin"
	"github.com/johnfercher/maroto/v2/pkg/core"
    "github.com/johnfercher/maroto/v2/pkg/components/col"
    "github.com/johnfercher/maroto/v2/pkg/components/line"
    "github.com/johnfercher/maroto/v2"
    "github.com/johnfercher/maroto/v2/pkg/components/code"
    "github.com/johnfercher/maroto/v2/pkg/components/signature"
    "github.com/johnfercher/maroto/v2/pkg/components/text"
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

// DB Drive Constant
const (
	dbDriver   = "mysql"
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
NDPRFileName = "ndpr.tmpl"
GDPRFileName = "gdpr.tmpl"
CCPAFileName = "ccpa.tmpl"
)

func main() {

    err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
		
	}
	


	token := os.Getenv("OPENAI_KEY")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")


	//open a database connection
	db, err := sql.Open(dbDriver, fmt.Sprintf("%s:%s@tcp(%s:3306)/%s", dbUser, dbPassword, dbHost, dbName))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	//Creat the "privacy_policies" table if it doesn't exist
	_, err = db.Exec(createTableQuery)
	if err != nil {
		log.Fatal(err)
	}

    //initialize OpenAI client
	client := openai.NewClient(token)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				     {
					        Role: openai.ChatMessageRoleUser,
							Content: "Hello!",
					 },
			},
		},
	)
	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return
	}
	fmt.Println(resp.Choices[0].Message.Content)

	// Initialize Gin

	router := gin.Default()

	//Set up a route to handle the web interface
	router.LoadHTMLGlob("templates/*")

	router.StaticFile("favicon.ico", "./favicon.ico" )

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", nil)
	})

			//create a New PDF document
	m := GetMaroto()
	document, err := m.Generate()
	if err != nil {
		log.Fatal(err)
	}
    
	router.GET("/download-pdf", func(c *gin.Context) {
		fNamer := fn.New()
		random := fNamer.Name()
		selectedPolicyType := c.Query("policyType")
		retrievedPolicy := PrivacyPolicy{} //Fetch the policy based on the type

		//Load the template and render the HTML content
		htmlContent := renderTemplate(loadTemplate(selectedPolicyType, selectedPolicyType+".tmpl"), retrievedPolicy)
		//Log generated output
		log.Printf("Generated Output: %v", htmlContent)



 
        //Save the document to the buffer
         err = document.Save(fmt.Sprintf("pdf/%s_privacy_policy_%s.pdf", selectedPolicyType, random))
        if err != nil {
            log.Fatal(err)
        }

		
		//Set up the HTTP response headers
        
        c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_privacy_policy_%s.pdf", selectedPolicyType, random))
        c.Header("Content-Type", "application/pdf")

		//Write the PDF content to the response
		http.ServeFile(c.Writer, c.Request, fmt.Sprintf("pdf/%s_privacy_policy_%s.pdf", selectedPolicyType, random))

		c.Status(http.StatusOK)



	})

	router.GET("/get-link", func(c *gin.Context) {
		//Link Creation implementation
	})

	router.POST("/generate", func(c *gin.Context) {
		//Parse form data
		var data PrivacyPolicy
		if err := c.ShouldBind(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
        


		// Sanitize user input
        data.CompanyName = sanitizeInput(data.CompanyName)
		data.Email = sanitizeInput(data.Email)
		data.Website = sanitizeInput(data.Website)
		data.Country = sanitizeInput(data.Country)
		data.RegistrationNumber = sanitizeInput(data.RegistrationNumber)
		data.Address = sanitizeInput(data.Address)

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

		renderedPolicies := map[string]template.HTML{
			selectedPolicyType: template.HTML(renderTemplate(loadTemplate(selectedPolicyType, selectedPolicyType+".tmpl"), retrievedPolicy)),
		}
		


		c.HTML(http.StatusOK, "generated_policy.tmpl", gin.H{
			"SelectedPolicy": selectedPolicyType,
			"PolicyContent": template.HTML(renderedPolicies[selectedPolicyType]),
		})


	})

	//Run the web server
	router.Run(":8080")
}


func sanitizeInput(input string) string {
	//Escape HTML characters
	sanitizedInput := html.EscapeString(input)
	//Trim leading and trailing whitespaces
	sanitizedInput = strings.TrimSpace(sanitizedInput)
	

	return sanitizedInput

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
	tmpl, err := template.New(name).ParseFiles("templates/base.tmpl", "templates/"+fileName)
	if err != nil {
		log.Fatal(err)
	}
	return tmpl
}


func GetMaroto() core.Maroto {

	m := maroto.New()
    m.AddRow(20,
        code.NewBarCol(4, "barcode"),
        code.NewMatrixCol(4, "matrixcode"),
        code.NewQrCol(4, "qrcode"),
    )

    m.AddRow(10, col.New(12))

    m.AddRow(20,
        //image.NewFromFileCol(4, "docs/assets/images/biplane.jpg"),
        signature.NewCol(4, "signature"),
        text.NewCol(4, "text"),
    )

    m.AddRow(10, col.New(12))

    m.AddRow(20, line.NewCol(12))

    return m
	//Testing New Version for versioning script
}

