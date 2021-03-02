package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/plutov/paypal/v3"
	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/checkout/session"
	"github.com/stripe/stripe-go/v72/webhookendpoint"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

func init() {
	stripe.Key = "sk_test_51IOvwAH6ZhYcgu7avCeOyGbPAKFv2EGxCDglpPcy5fmnZ0dZ4SySbhMYHWg6ryYOiyFET2VMz8KLEdb5HQSoOUMg008xHEPkvc"
}

func main() {
	clientID := "AYctwv37z9ZuxOIaTRdM_hnC_5Y839gKpj7oelVYgxtwLU0loI9z0W7AmdVouqp85Sj_UZfHyVp8G6xb"
	clientSecret := "ENV6l5QmnP9Myf57RWCSpUdzmMf01WtYLXuy6M0iMwde7voNUqX-QKtY4KZvZHToaeuOsZtqZWVt51MD"

	/*clientID := "ASLttKbHgWuOIl45HNgqBNWnVJYWLugXa1mTaICKu2yj7ZWWoh2sSeHB0LhjFMFCFKD1H0DmCFYpKxtI"
	clientSecret := "EKUs6gQXA-tBh5QMqYXABPbmJ-obYZ2MIOhUWU-Nib_3RQ7v1kVf5sNYvvmSl9WgFX2ZTgYd-XB89gJl"*/

	r := gin.Default()
	r.Use(CORS())
	r.POST("/payment-stripe", func(context *gin.Context) {
		params := &stripe.CheckoutSessionParams{
			PaymentMethodTypes: stripe.StringSlice([]string{
				"card",
			}),
			LineItems: []*stripe.CheckoutSessionLineItemParams{
				&stripe.CheckoutSessionLineItemParams{
					PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
						Currency: stripe.String(string(stripe.CurrencyUSD)),
						ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
							Name: stripe.String("T-shirt"),
							Metadata: map[string]string{
								"license_id":       "1",
								"corporation_name": "TMA Solutions",
							},
						},
						UnitAmount: stripe.Int64(2000),
					},
					Quantity: stripe.Int64(1),
				},
			},
			Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
			SuccessURL: stripe.String("https://google.com"),
			CancelURL:  stripe.String("https://youtube.com"),
		}
		session, err := session.New(params)
		if err != nil {
			log.Printf("session.New: %v", err)
		}
		context.JSON(200, gin.H{
			"id": session.ID,
		})
	})

	r.POST("/stripe-webhook", func(context *gin.Context) {
		params := &stripe.WebhookEndpointParams{
			EnabledEvents: []*string{
				stripe.String("charge.failed"),
				stripe.String("charge.succeeded"),
			},
			URL: stripe.String("https://example.com/my/webhook/endpoint"),
		}
		we, _ := webhookendpoint.New(params)
		context.JSON(200, we)
	})

	r.POST("/payment-paypal", func(c *gin.Context) {
		client, err := paypal.NewClient(clientID, clientSecret, paypal.APIBaseLive)
		if err != nil {
			c.JSON(200, err.Error())
			return
		}
		// Retrieve access token
		_, err = client.GetAccessToken()
		if err != nil {
			panic(err)
		}
		order, err := client.CreateOrder("CAPTURE", []paypal.PurchaseUnitRequest{
			{
				Amount: &paypal.PurchaseUnitAmount{
					Currency: "USD",
					Value:    "100.00",
					Breakdown: &paypal.PurchaseUnitAmountBreakdown{
						ItemTotal: &paypal.Money{
							Currency: "USD",
							Value:    "100.00",
						},
					},
				},
				Items: []paypal.Item{
					{
						Name:     "Renew License",
						Quantity: "1",
						UnitAmount: &paypal.Money{
							Currency: "USD",
							Value:    "100.00",
						},
					},
				},
			},
		},
			&paypal.CreateOrderPayer{

			},
			&paypal.ApplicationContext{ReturnURL: "https://google.com", CancelURL: "https:/youtube.com"},
		)
		fmt.Println(order)
		c.JSON(http.StatusOK, gin.H{
			"id": order.ID,
		})
	})

	r.POST("/payment-paypal-execute/:orderID", func(context *gin.Context) {
		orderID := context.Param("orderID")
		body := context.Request.Body
		payload, _ := ioutil.ReadAll(body)

		payment := struct {
			PayerID      string
			PaymentToken string
		}{}

		payment.PayerID = strings.Split(strings.Split(string(payload), "&")[0], "=")[1]
		payment.PaymentToken = strings.Split(strings.Split(string(payload), "&")[1], "=")[1]

		client, err := paypal.NewClient(clientID, clientSecret, paypal.APIBaseLive)
		if err != nil {
			context.JSON(200, err.Error())
			return
		}
		// Retrieve access token
		_, err = client.GetAccessToken()
		if err != nil {
			panic(err)
		}

		// capture order
		order, err := client.GetOrder(orderID)
		if err != nil {
			return
		}

		captureLink := ""
		for _, link := range order.Links {
			if link.Method == "POST" && link.Rel == "capture" {
				captureLink = link.Href
				break
			}
		}

		reqPaymentBytes, err := json.Marshal(payment)
		if err != nil {
			return
		}

		paymentReader := bytes.NewReader(reqPaymentBytes)

		req, err := http.NewRequest("POST", captureLink, paymentReader)
		if err != nil {
			return
		}
		req.Header.Set("Authorization", "Bearer " + client.Token.Token)
		req.Header.Set("Content-Type", "application/json; charset=utf8")
		req.Header.Set("Accept", "*/*")
		res, err := client.Client.Do(req)
		if err != nil {
			return
		}

		buf := new(bytes.Buffer)
		buf.ReadFrom(res.Body)

		context.JSON(http.StatusOK, buf.String())
	})

	r.Run("localhost:8080")
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, lang, x-tenant")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
			return
		}
		c.Next()
	}
}
