package main_test

import (
	"os"
	"testing"

	. "github.com/MorpheoOrg/morpheo-compute/api"
	"gopkg.in/kataras/iris.v6"
	"gopkg.in/kataras/iris.v6/httptest"

	"github.com/MorpheoOrg/morpheo-go-packages/common"
)

var (
	app              *iris.Framework
	valid_learnuplet = map[string]interface{}{
		"algo":               "0885fe91-da5a-4896-988f-3625b53b38b9",
		"model_end":          "3ce43ff0-c602-402b-823f-056ad8b4f28f",
		"model_start":        "0885fe91-da5a-4896-988f-3625b53b38b9",
		"perf":               nil,
		"problem":            "2869781a-c481-4ed7-b88a-a5073bae8326",
		"rank":               1,
		"status":             "todo",
		"test_data":          []string{"6c619a93-5989-4153-90b8-ba93328ebc5f", "2ee0dd40-2fe7-402a-a128-c47204a6a5a0"},
		"test_perf":          nil,
		"timestamp_creation": 1508514453,
		"timestamp_done":     nil,
		"train_data":         []string{"8436d362-fe38-4d35-96c2-4496451758cf"},
		"train_perf":         nil,
		"uuid":               "3473ce23-25da-48da-9803-65cfefc1f59d",
		"worker":             nil,
		"workflow":           "e568587d-572c-4714-8084-378ed50d1c52",
	}
	valid_preduplet = map[string]interface{}{
		"data":              "8436d362-fe38-4d35-96c2-4496451758cf",
		"model":             "6240ea48-cc46-4d46-bc26-e0bcce6fcd58",
		"problem":           "2869781a-c481-4ed7-b88a-a5073bae8326",
		"status":            "todo",
		"timestamp_done":    nil,
		"timestamp_request": nil,
		"uuid":              "0ed11e3f-e307-499e-be16-996cf3949653",
		"worker":            nil,
		"workflow":          "e568587d-572c-4714-8084-378ed50d1c52",
	}
)

func TestMain(m *testing.M) {
	// Set test App
	conf := NewProducerConfig()
	producer := &common.ProducerMOCK{}
	app = SetIrisApp(conf, producer)
	os.Exit(m.Run())
}

// Test index request returns Sucess
func TestIndexRoute(t *testing.T) {
	e := httptest.New(app, t)
	e.GET(RootRoute).Expect().Status(200)
}

// Test health request returns Status Ok.
func TestHealthRoute(t *testing.T) {
	e := httptest.New(app, t)
	e.GET(HealthRoute).Expect().Status(200).JSON().Equal(map[string]interface{}{"status": "ok"})
}

func TestPostLearnuplet(t *testing.T) {
	e := httptest.New(app, t)

	// Test valid Learnuplet returns StatusAccepted
	e.POST(LearnRoute).WithJSON(valid_learnuplet).Expect().Status(202)

	// Test wrong Learnuplet returns BadRequest
	wrong_learnuplet := valid_learnuplet
	wrong_learnuplet["status"] = "xxx"
	e.POST(LearnRoute).WithJSON(wrong_learnuplet).Expect().Status(400).Body().Match("(.*)Invalid learn-uplet(.*)")

	// Test wrong serialization returns BadRequest
	wrong_learnuplet["problem"] = ""
	e.POST(LearnRoute).WithJSON(wrong_learnuplet).Expect().Status(400).Body().Match("(.*)Error decoding body to JSON(.*)")
}

func TestPostPreduplet(t *testing.T) {
	e := httptest.New(app, t)

	// Test valid Preduplet returns StatusAccepted
	e.POST(PredRoute).WithJSON(valid_preduplet).Expect().Status(202)

	// Test wrong Preduplet returns BadRequest
	wrong_preduplet := valid_preduplet
	wrong_preduplet["status"] = "xxx"
	e.POST(PredRoute).WithJSON(wrong_preduplet).Expect().Status(400).Body().Match("(.*)Invalid pred-uplet(.*)")

	// Test wrong serialization returns BadRequest
	wrong_preduplet["problem"] = ""
	e.POST(PredRoute).WithJSON(wrong_preduplet).Expect().Status(400).Body().Match("(.*)Error decoding body to JSON(.*)")
}
