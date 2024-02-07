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
	"regexp"
    
	openai "github.com/sashabaranov/go-openai"
	"github.com/joho/godotenv"
    "github.com/bcongdon/fn"
	"github.com/gin-gonic/gin"
	"github.com/johnfercher/maroto/v2/pkg/core"
    "github.com/johnfercher/maroto/v2"
    "github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
    "github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
    "github.com/johnfercher/maroto/v2/pkg/props"
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
	Content            string
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

//shared variable to store lastInsertID
var lastInsertID int

// Create the "privacy_policies" table if it doesn't exist

const createTableQuery = `
CREATE TABLE IF NOT EXISTS privacy_policies (
	id INT AUTO_INCREMENT PRIMARY KEY,
	company_name VARCHAR(255) NOT NULL,
	email VARCHAR(255) NOT NULL,
	website VARCHAR(255) NOT NULL,
	country VARCHAR(255) NOT NULL,
	registration_number VARCHAR(255) NOT NULL,
	address VARCHAR(255) NOT NULL,
	content LONGTEXT NOT NULL

);
`



// Insert a privacy policy into the database
const insertQuery = `
INSERT INTO privacy_policies (company_name, email, website, country, registration_number, address, content)
VALUES (?, ?, ?, ?, ?, ?, ?);
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
	




	//open a database connection
	db, err := sql.Open(dbDriver, fmt.Sprintf("%s:%s@tcp(%s:3306)/%s", os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_HOST"), os.Getenv("DB_NAME")))
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

	router.StaticFile("favicon.ico", "./favicon.ico" )

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", nil)
	})

			//create a New PDF document
	
    
	router.GET("/download-pdf", func(c *gin.Context) {
		fNamer := fn.New()
		random := fNamer.Name()
		selectedPolicyType := c.Query("policyType")

		if selectedPolicyType == "" {
			log.Println("Policy Type not provided")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Policy type not provided"})
		}
		//add lastInsertID as query parameter
		lastInsertID := c.Query("lastInsertID")
		if lastInsertID == "" {
			log.Println("lastInsertID not provided")
			c.JSON(http.StatusBadRequest, gin.H{"error": "lastInsertID not provided"})
			return
		} 

		var policyContent string
		err = db.QueryRow("SELECT content FROM privacy_policies WHERE id = ?", lastInsertID).Scan(&policyContent)
        if err != nil {
			log.Println("Error retrieving policy content from database:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving policy content from database"})
			return
        }

		m := GetMaroto(policyContent, selectedPolicyType)
		document, err := m.Generate()
		if err != nil {
			log.Println("Error generating maroto pdf file", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating maroto pdf file"})
			return
		}

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

		//Check if all required fields are present
		if data.CompanyName == "" || data.Email == "" || data.Website == "" || data.Country == "" || data.RegistrationNumber == "" || data.Address == "" {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Please fill in all required fields"})
			return
		}


		// Sanitize user input
        data.CompanyName = sanitizeInput(data.CompanyName)
		data.Email = sanitizeInput(data.Email)
		data.Website = sanitizeInput(data.Website)
		data.Country = sanitizeInput(data.Country)
		data.RegistrationNumber = sanitizeInput(data.RegistrationNumber)
		data.Address = sanitizeInput(data.Address)
		data.Content = " "

		// Insert the privacy policy into the database
		result, err := db.Exec(insertQuery, data.CompanyName, data.Email, data.Website, data.Country, data.RegistrationNumber, data.Address, data.Content)
		if err != nil {
			log.Println("Error Inserting Privacy Policy into database")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Retrive the inserted privacy policy by ID
		lastInsertID, _ := result.LastInsertId() //Store lastInsertID in the precreated variable
		retrievedPolicy := PrivacyPolicy{}
		err = db.QueryRow(selectQuery, lastInsertID).Scan(
			&retrievedPolicy.ID,
			&retrievedPolicy.CompanyName,
			&retrievedPolicy.Email,
			&retrievedPolicy.Website,
			&retrievedPolicy.Country,
			&retrievedPolicy.RegistrationNumber,
			&retrievedPolicy.Address,
			&retrievedPolicy.Content,
		)

		if err != nil {
			log.Println("Error retrieving inserted privacy policy by id")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

        
            //initialize OpenAI client
	    client := openai.NewClient(os.Getenv("OPENAI_KEY"))
	    resp, err := client.CreateChatCompletion(
		    context.Background(),
		    openai.ChatCompletionRequest{
			    Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				     {
					        Role: openai.ChatMessageRoleUser,
							Content: "Generate a GDPR compliant privacy policy for my company, Disrupt technologies",
					 },
			},
		},
	   )
	    if err != nil {
		    fmt.Printf("ChatCompletion error: %v\n", err)
		    c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		    return
	    }
	    openaiContent := resp.Choices[0].Message.Content

		//log.Printf("Generated Policy: %v", openaiContent)
		
		log.Printf("lastInsertID: %v", lastInsertID)

		// update database with content
		_, err = db.Exec("UPDATE privacy_policies SET content = ? WHERE ID = ?", openaiContent, lastInsertID)
		if err != nil {
			log.Println("Error updating database when generated policy")
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
			"OpenAIContent": openaiContent, // add OpenAI content to the response
			"LastInsertID": lastInsertID,
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
	//log.Printf("Generated HTML: %v", resultString)
	return resultString
}


func loadTemplate(name, fileName string) *template.Template {
	tmpl, err := template.New(name).ParseFiles("templates/base.tmpl", "templates/"+fileName)
	if err != nil {
		log.Fatal(err)
	}
	return tmpl
}


func GetMaroto(policyContent string, selectedPolicyType string ) core.Maroto {

	m := maroto.New()
    
	header := " Compliant Privacy Policy"

	selectedPolicyType += ""

	selectedPolicyType = strings.ToUpper(selectedPolicyType)

	selectedPolicyType += header

	m.AddRow(10, text.NewCol(10, selectedPolicyType, props.Text{
        Size:  24,
        Style: fontstyle.Bold,
        Align: align.Center,
    })) 
		
    m.AddRow(25,)


	    // Define regex pattern to match any number followed by a full stop
	pattern := `\d+\.` // This pattern matches any number followed by a full stop

		// Compile the regex pattern
	regex := regexp.MustCompile(pattern)
	
		// Replace any number followed by a full stop with an empty string
	policyContent = regex.ReplaceAllString(policyContent, "\n")

	log.Printf("Pre-Rendered Output: %v", policyContent)

	paragraphs := strings.Split(policyContent, "  ")

    for _, paragraph := range paragraphs {
        m.AddRow(20, text.NewCol(12, paragraph, props.Text{
            Size: 14,
        }))
    }

    return m
}

