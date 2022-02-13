package main

import (
	"context"
	"crypto/tls"
	"log"
	"os"
	"regexp"
	"time"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/gomail.v2"
)

var ctx = context.TODO()
var client = ConnectDB()
var loggerColl = client.Database("databases").Collection("loggers")
var userColl = client.Database("databases").Collection("users")

type visitor struct {
	IPs      []string
	Clicked time.Time
}

type logger struct {
	Id        string
	Url       string
	Dashboard string
	ApiKey    string
	Visitors  []visitor
	Clicks    int
}

type user struct {
	Email  string
	ApiKey string
}

func ConnectDB() *mongo.Client {

	godotenv.Load()

	// err := godotenv.Load()
	// if err != nil {
	// 	panic(err)
	// }
	
	var dbUser string = os.Getenv("MONGO_USER")
	var dbPass string = os.Getenv("MONGO_PSW")
	var dbDm string = os.Getenv("MONGO_DM")

	var uri string = "mongodb+srv://" + dbUser + ":" + dbPass + "@"+ dbDm + "/?retryWrites=true&w=majority"

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))

	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	return client
}

func createLogger(logger *logger) error {
	_, err := loggerColl.InsertOne(ctx, &logger)
	return err
}

func findLoggerByUrl(url string) (*logger, error) {
	var logger logger

	err := loggerColl.FindOne(ctx, bson.M{"url": url}).Decode(&logger)
	if err != nil {
		return nil, err
	}

	return &logger, nil
}

func findLoggerById(id string) (*logger, error) {
	var logger logger

	err := loggerColl.FindOne(ctx, bson.M{"id": id}).Decode(&logger)
	if err != nil {
		return nil, err
	}

	return &logger, nil
}

func findUserByApiKey(apiKey string) (bool, string) {
	var user user

	err := userColl.FindOne(ctx, bson.M{"apikey": apiKey}).Decode(&user)
	if err != nil {
		return false, ""
	}

	return true, user.Email
}

func findUserByEmail(email string) (bool, string) {
	var user user

	err := userColl.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return false, ""
	}

	return true, user.ApiKey
}

// func createAccount() error {

// }

func isEmailValid(e string) bool {
	emailRegex := regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	return emailRegex.MatchString(e)
}

func main() {

	var mailPsw string = os.Getenv("MAIL_PSW")
	var ownMail string = os.Getenv("MAIL")

	app := fiber.New()

	d := gomail.NewDialer("smtp.gmail.com", 587, ownMail, mailPsw)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	app.Post("/api/getApiKey", func(c *fiber.Ctx) error {

		email := c.FormValue("email")

		if !isEmailValid(email) {
			return c.Status(400).JSON(&fiber.Map{"message": "Invalid email"})
		}

		if ok, apiKey := findUserByEmail(email); ok {

			m := gomail.NewMessage()
			m.SetHeader("From", "apikey@ipshorter.dev")
			m.SetHeader("To", email)
			m.SetHeader("Subject", "ApiKey | IPShorter")
			m.SetBody("text/html", "This is your ApiKey <b>"+apiKey+"</b>")

			return c.Status(400).JSON(&fiber.Map{"error": "User already have an apiKey , but we resent it to you by email"})
		}

		apiKey := uuid.New().String()

		m := gomail.NewMessage()
		m.SetHeader("From", "apikey@ipshorter.dev")
		m.SetHeader("To", email)
		m.SetHeader("Subject", "ApiKey | IPShorter")
		m.SetBody("text/html", "This is your ApiKey <b>"+apiKey+"</b>")

		if err := d.DialAndSend(m); err != nil {
			panic(err)
		}

		userColl.InsertOne(context.TODO(), &user{Email: email, ApiKey: apiKey})

		return c.Status(200).JSON(&fiber.Map{"message": "ApiKey sent to your email"})

	})

	app.Post("/api/createLogger/:apiKey", func(c *fiber.Ctx) error {

		apiKey := c.Params("apiKey")
		url := c.FormValue("url")

		if url == "" {
			return c.Status(400).JSON(&fiber.Map{"error": "url is required"})
		} else if exLogger, _ := findLoggerByUrl(url); exLogger != nil {

			return c.Status(400).JSON(&fiber.Map{"error": "url is already in use"})

		}

		if ok, _ := findUserByApiKey(apiKey); !ok {
			return c.Status(400).JSON(&fiber.Map{"error": "apiKey does not exist", "apiKey": apiKey})
		}

		idDashboard := uuid.New().String()
		idLogger := uuid.New().String()

		type response struct {
			Id        string
			Url       string
			Dashboard string
		}

		urlLogger := &logger{
			Id:        idLogger,
			Dashboard: idDashboard,
			Url:       url,
			ApiKey:    apiKey,
		}

		if err := createLogger(urlLogger); err != nil {
			return c.Status(400).JSON(&fiber.Map{"error": "error creating logger"})
		}

		res := &response{
			Id:        idLogger,
			Url:       url,
			Dashboard: idDashboard,
		}

		return c.JSON(res)
	})

	app.Get("/api/getLoggers/:apiKey", func(c *fiber.Ctx) error {

		apiKey := c.Params("apiKey")

		var loggers []logger

		cur, err := loggerColl.Find(ctx, bson.M{"apikey": apiKey})

		if err != nil {
			return c.Status(400).JSON(&fiber.Map{"error": "error getting loggers"})
		}

		for cur.Next(ctx) {
			var logger logger
			err := cur.Decode(&logger)
			if err != nil {
				return c.Status(400).JSON(&fiber.Map{"error": "error getting loggers"})
			}

			loggers = append(loggers, logger)
		}

		return c.JSON(loggers)

	})

	app.Get("/:id", func(c *fiber.Ctx) error {

		id := c.Params("id")

		logger, err := findLoggerById(id)

		if err != nil {
			return c.Status(400).JSON(&fiber.Map{"error": "error getting the destination url"})
		}

		logger.Clicks++
		loggerColl.UpdateOne(ctx, bson.M{"id": id}, bson.M{"$set": bson.M{"clicks": logger.Clicks}})

		userIP := c.IPs()

		visitor := visitor{
			IPs:      userIP,
			Clicked: time.Now(),
		}

		visitors := append(logger.Visitors, visitor)

		loggerColl.UpdateOne(ctx, bson.M{"id": id}, bson.M{"$set": bson.M{"visitors": visitors}})

		return c.Redirect(logger.Url)

	})

	app.Get("/api/dashboard/:apiKey/:id", func(c *fiber.Ctx) error {

		apiKey := c.Params("apiKey")
		id := c.Params("id")

		cur, err := loggerColl.Find(ctx, bson.M{"apikey": apiKey})

		if err != nil {
			return c.Status(400).JSON(&fiber.Map{"error": "error getting loggers"})
		}

		for cur.Next(ctx) {
			var logger logger
			err := cur.Decode(&logger)
			if err != nil {
				return c.Status(400).JSON(&fiber.Map{"error": "error getting loggers"})
			}

			if logger.Id == id {
				return c.JSON(logger)
			}
		}

		return c.Status(400).JSON(&fiber.Map{"error": "error getting loggers"})

	})

	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	err := app.Listen(":" + port)

	if err != nil {
		log.Fatal(err)
	}

}
