package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	milvus "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	config "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan/config"
	plugins "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan/plugins"
)

var Plugin plugins.Plugin = &Memory{}

type Memory struct {
	cfg          config.Cfg
	milvusClient milvus.Client
	openaiClient *openai.Client
}

type memory struct {
	Memory string
	Vector []float32
}

type memoryResult struct {
	Memory string
	Type   string
	Detail string
	Score  float32
}

type memoryItem struct {
	Memory string `json:"memory"`
	Type   string `json:"type"`
	Detail string `json:"detail"`
}

type inputDefinition struct {
	RequestType  string       `json:"requestType"`
	Memories     []memoryItem `json:"memories"`
	Num_relevant int          `json:"num_relevant"`
}

func (c *Memory) Init(cfg config.Cfg, openaiClient *openai.Client) error {
	c.cfg = cfg
	c.openaiClient = openaiClient

	ctx := context.Background()

	c.milvusClient, _ = milvus.NewGrpcClient(ctx, c.cfg.MalvusApiEndpoint())

	err := c.initMilvusSchema()

	if err != nil {
		fmt.Println("Error initializing Milvus schema: ", err)
		return err
	}

	fmt.Println("Memory plugin initialized successfully")
	return nil
}

func (c Memory) ID() string {
	return "memory"
}

func (c Memory) Description() string {
	return "store and retrieve memories from long term memory."
}

func (c Memory) FunctionDefinition() openai.FunctionDefinition {
	return openai.FunctionDefinition{
		Name:        "memory",
		Description: "从长期记忆中存储和检索记忆。使用requestType 'set'向数据库添加记忆，使用requestType 'get'检索最相关的记忆。首次启动时，你应该使用'hydrate'功能回顾用户的过往记忆。",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"requestType": {
					Type:        jsonschema.String,
					Description: "要进行的请求类型 'set'，'get' 或 'hydrate'。'set' 将记忆添加到数据库中，'get' 将返回最相关的记忆。获取记忆时，你应该总是包含记忆字段。'hydrate'将返回包含用户所有记忆的提示。",
				},
				"memories": {
					Type: jsonschema.Array,
					Items: &jsonschema.Definition{
						Type: jsonschema.Object,
						Properties: map[string]jsonschema.Definition{
							"memory": {
								Type:        jsonschema.String,
								Description: "要添加的个别记忆。你应该提供尽可能多的上下文来配合记忆。",
							},
							"type": {
								Type:        jsonschema.String,
								Description: "记忆的类型，例如：'personality', 'food'等。",
							},
							"detail": {
								Type:        jsonschema.String,
								Description: "关于类型的具体细节，例如：'喜欢披萨'，'是风情万种的'等。",
							},
						},
						Required: []string{"memory", "type", "detail"},
					},
					Description: "要添加或获取的记忆数组。每个记忆包含其个别内容、类型和细节。这对于'set'和'get'请求都是必需的。",
				},
				"num_relevant": {
					Type:        jsonschema.Integer,
					Description: "要返回的相关记忆的数量，例如：5。",
				},
			},
			Required: []string{"requestType"},
		},
	}
}

func (c Memory) Execute(jsonInput string) (string, error) {
	// marshal jsonInput to inputDefinition
	var args inputDefinition
	err := json.Unmarshal([]byte(jsonInput), &args)
	if err != nil {
		fmt.Println("Error unmarshalling JSON input: ", err)
		return "", err
	}

	if args.Num_relevant == 0 {
		args.Num_relevant = 5
	}

	// Check if memories slice is empty
	if args.RequestType != "hydrate" && len(args.Memories) == 0 {
		return fmt.Sprintf(`%v`, "memories are required but was empty"), nil
	}

	switch args.RequestType {
	case "set":
		// Iterate over all memories and set them
		for _, memory := range args.Memories {
			ok, err := c.setMemory(memory.Memory, memory.Type, memory.Detail)
			if err != nil {
				fmt.Println("Error setting memory: ", err)
				return fmt.Sprintf(`%v`, err), err
			}
			if !ok {
				return "Failed to set a memory", nil
			}
		}
		fmt.Println("Memories set successfully")
		return "Memories set successfully", nil

	case "get":
		// Note: This assumes that for 'get', you'll retrieve memories based on the first item in the memories slice. Adjust as needed.
		memoryResponse, err := c.getMemory(args.Memories[0], args.Num_relevant)
		if err != nil {
			fmt.Println("Error getting memory: ", err)
			return fmt.Sprintf(`%v`, err), err
		}
		fmt.Println("Memories get successfully")
		return fmt.Sprintf(`%v`, memoryResponse), nil
	case "hydrate":
		prompt, err := c.HydrateUserMemories()
		if err != nil {
			fmt.Println("Error hydrating user memories: ", err)
			return fmt.Sprintf(`%v`, err), err
		}
		fmt.Println("Memories hydrate successfully")
		return prompt, nil
	default:
		return "unknown request type check out Example for how to use the memory plug", nil
	}
}

func (c Memory) getEmbeddingsFromOpenAI(data string) openai.Embedding {
	embeddings, err := c.openaiClient.CreateEmbeddings(context.Background(), openai.EmbeddingRequest{
		Input: []string{data},
		Model: openai.AdaEmbeddingV2,
	})
	if err != nil {
		fmt.Println("Error getting embeddings from OpenAI: ", err)
		fmt.Println(err)
	}

	return embeddings.Data[0]
}

func (c Memory) setMemory(newMemory, memoryType, memoryDetail string) (bool, error) {
	// Step 1: Combine the three fields into a single string
	combinedMemory := memoryType + "|" + memoryDetail + "|" + newMemory

	embeddings := c.getEmbeddingsFromOpenAI(combinedMemory)

	longTermMemory := memory{
		Memory: combinedMemory, // Use combinedMemory here
		Vector: embeddings.Embedding,
	}

	memories := []memory{
		longTermMemory,
	}

	memoryData := make([]string, 0, len(memories))
	vectors := make([][]float32, 0, len(memories))

	for _, memory := range memories {
		memoryData = append(memoryData, memory.Memory)
		vectors = append(vectors, memory.Vector)
	}

	memoryColumn := entity.NewColumnVarChar("memory", memoryData)
	vectorColumn := entity.NewColumnFloatVector("embeddings", 1536, vectors)

	_, err := c.milvusClient.Insert(context.Background(), c.cfg.MalvusCollectionName(), "", memoryColumn, vectorColumn)

	if err != nil {
		fmt.Println("Error inserting into Milvus client: ", err)
		return false, err
	}

	return true, nil
}

func (c Memory) getMemory(memory memoryItem, num_relevant int) ([]memoryResult, error) {
	combinedMemory := memory.Type + "|" + memory.Detail + "|" + memory.Memory + ","
	embeddings := c.getEmbeddingsFromOpenAI(combinedMemory)

	ctx := context.Background()
	partitions := []string{}
	expr := ""
	outputFields := []string{"memory"}
	vectors := []entity.Vector{entity.FloatVector(embeddings.Embedding)}
	vectorField := "embeddings"
	metricType := entity.L2
	topK := num_relevant

	searchParam, _ := entity.NewIndexFlatSearchParam()

	options := []milvus.SearchQueryOptionFunc{}

	searchResult, err := c.milvusClient.Search(ctx, c.cfg.MalvusCollectionName(), partitions, expr, outputFields, vectors, vectorField, metricType, topK, searchParam, options...)

	if err != nil {
		fmt.Println("Error searching in Milvus client: ", err)
		return nil, err
	}

	memoryFields := c.getStringSliceFromColumn(searchResult[0].Fields.GetColumn("memory"))

	var allMemories []string
	if len(memoryFields) == 1 {
		allMemories = strings.Split(memoryFields[0], ",")
	} else {
		allMemories = memoryFields
	}

	memoryResults := make([]memoryResult, len(allMemories))

	for idx, memory := range allMemories {
		parts := strings.Split(memory, "|")

		if len(parts) >= 3 {
			memoryResults[idx] = memoryResult{
				Type:   strings.TrimSpace(parts[0]),
				Detail: strings.TrimSpace(parts[1]),
				Memory: strings.TrimSpace(parts[2]),
			}
		}
	}

	return memoryResults, nil
}

func (c Memory) getStringSliceFromColumn(column entity.Column) []string {
	length := column.Len()
	results := make([]string, length)

	for i := 0; i < length; i++ {
		val, err := column.GetAsString(i)
		if err != nil {
			// handle error or continue with a placeholder value
			fmt.Println("Error getting string from column: ", err)
			results[i] = "" // or some placeholder value
		} else {
			results[i] = val
		}
	}

	return results
}

func (c Memory) initMilvusSchema() error {

	//check if schema exists

	if exists, _ := c.milvusClient.HasCollection(context.Background(), c.cfg.MalvusCollectionName()); !exists {
		schema := &entity.Schema{
			CollectionName: c.cfg.MalvusCollectionName(),
			Description:    "Clara's long term memory",
			Fields: []*entity.Field{
				{
					Name:       "memory_id",
					DataType:   entity.FieldTypeInt64,
					PrimaryKey: true,
					AutoID:     true,
				},
				{
					Name:     "memory",
					DataType: entity.FieldTypeVarChar,
					TypeParams: map[string]string{
						entity.TypeParamMaxLength: "65535",
					},
				},
				{
					Name:     "embeddings",
					DataType: entity.FieldTypeFloatVector,
					TypeParams: map[string]string{
						entity.TypeParamDim: "1536",
					},
				},
			},
		}
		err := c.milvusClient.CreateCollection(context.Background(), schema, 1)
		if err != nil {
			fmt.Println("Error creating collection in Milvus client: ", err)
			return err
		}

		idx, err := entity.NewIndexIvfFlat(entity.L2, 2)

		if err != nil {
			fmt.Println("Error creating index in Milvus client: ", err)
			return err
		}

		err = c.milvusClient.CreateIndex(context.Background(), c.cfg.MalvusCollectionName(), "embeddings", idx, false)

		if err != nil {
			fmt.Println("Error creating index in Milvus client: ", err)
			return err
		}

	}

	//check to see if the collection is loaded
	loaded, err := c.milvusClient.GetLoadState(context.Background(), c.cfg.MalvusCollectionName(), []string{})

	if err != nil {
		fmt.Println("Error getting load state from Milvus client: ", err)
		return err
	}

	if loaded == entity.LoadStateNotLoad {
		err = c.milvusClient.LoadCollection(context.Background(), c.cfg.MalvusCollectionName(), false)
		if err != nil {
			fmt.Println("Error loading collection from Milvus client: ", err)
			return err
		}
	}

	return nil
}

func (c *Memory) HydrateUserMemories() (string, error) {

	var memories = []memoryItem{
		{Type: "Basic Personal Information", Detail: "name"},
		{Type: "Basic Personal Information", Detail: "age"},
		{Type: "Basic Personal Information", Detail: "gender"},
		{Type: "Basic Personal Information", Detail: "location"},

		{Type: "Preferences", Detail: "music_preference"},
		{Type: "Preferences", Detail: "movie_preference"},
		{Type: "Preferences", Detail: "book_preference"},
		{Type: "Preferences", Detail: "food_preference"},

		{Type: "Professional and Educational Background", Detail: "profession"},
		{Type: "Professional and Educational Background", Detail: "education"},
		{Type: "Professional and Educational Background", Detail: "skills"},

		{Type: "Hobbies and Interests", Detail: "hobbies"},
		{Type: "Hobbies and Interests", Detail: "sports"},
		{Type: "Hobbies and Interests", Detail: "travel"},
		{Type: "Hobbies and Interests", Detail: "games"},

		{Type: "Lifestyle and Habits", Detail: "exercise_habit"},
		{Type: "Lifestyle and Habits", Detail: "reading_habit"},
		{Type: "Lifestyle and Habits", Detail: "diet"},
		{Type: "Lifestyle and Habits", Detail: "pets"},

		{Type: "Tech and Media Consumption", Detail: "favorite_apps"},
		{Type: "Tech and Media Consumption", Detail: "device_preference"},
		{Type: "Tech and Media Consumption", Detail: "news_source"},

		{Type: "Social and Personal Relationships", Detail: "family"},
		{Type: "Social and Personal Relationships", Detail: "friends"},
		{Type: "Social and Personal Relationships", Detail: "relationship_status"},

		{Type: "Past Interactions", Detail: "past_questions"},
		{Type: "Past Interactions", Detail: "feedback"},
		{Type: "Past Interactions", Detail: "topics_of_interest"},

		{Type: "Moods and Feelings", Detail: "current_mood"},
		{Type: "Moods and Feelings", Detail: "life_events"},
		{Type: "Moods and Feelings", Detail: "challenges"},

		{Type: "Custom User Data", Detail: "custom_data"},
	}

	uniqueMemories := make(map[string]bool)
	var memoryList []string

	prompt := "你是一个名叫小丸的AI助手，你拥有长期记忆，以下是一些关于用户的记忆，你可以使用："

	for _, m := range memories {
		// Get each memory from the vector database based on user ID and memory type
		results, err := c.getMemory(m, 5)
		if err != nil {
			return "", err
		}

		for _, res := range results {
			// Remove the trailing comma from each memory if it exists
			cleanMemory := strings.TrimSuffix(res.Memory, ",")
			if _, exists := uniqueMemories[cleanMemory]; !exists {
				uniqueMemories[cleanMemory] = true
				memoryList = append(memoryList, cleanMemory)
			}
		}
	}

	// Convert unique memories into a CSV list
	memoriesCSV := strings.Join(memoryList, ", ")

	prompt += memoriesCSV

	return prompt, nil
}
