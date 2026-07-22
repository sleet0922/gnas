package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDuplicateGroupingDoesNotChainUnrelatedImages(t *testing.T) {
	oldDataDir := dataDir
	oldQdrantURL := qdrantURL
	dataDir = t.TempDir()
	defer func() {
		dataDir = oldDataDir
		qdrantURL = oldQdrantURL
	}()

	ids := []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
		"00000000-0000-0000-0000-000000000003",
	}
	points := make([]qdrantPoint, len(ids))
	for index, id := range ids {
		imagePath := filepath.Join(dataDir, string(rune('a'+index))+".jpg")
		if err := os.WriteFile(imagePath, []byte("image"), 0644); err != nil {
			t.Fatal(err)
		}
		points[index].ID = id
		points[index].Vector = make([]float32, embeddingDimension)
		points[index].Vector[0] = float32(index + 1)
		points[index].Payload.Path = imagePath
	}

	qdrant := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Query []float32 `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		neighbors := []qdrantNearestPoint{}
		switch int(request.Query[0]) {
		case 1:
			neighbors = []qdrantNearestPoint{{ID: ids[0], Score: 1}, {ID: ids[1], Score: 0.9}}
		case 2:
			neighbors = []qdrantNearestPoint{{ID: ids[1], Score: 1}, {ID: ids[0], Score: 0.9}, {ID: ids[2], Score: 0.9}}
		case 3:
			neighbors = []qdrantNearestPoint{{ID: ids[2], Score: 1}, {ID: ids[1], Score: 0.9}}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{"points": neighbors},
		})
	}))
	defer qdrant.Close()
	qdrantURL = qdrant.URL

	groups, err := calculateDuplicateGroups(points)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(groups))
	}
	if len(groups[0].Items) != 2 {
		t.Fatalf("group items = %d, want 2; unrelated images were chained", len(groups[0].Items))
	}
	if groups[0].Similarity < 0.899 || groups[0].Similarity > 0.901 {
		t.Fatalf("group similarity = %f, want 0.9", groups[0].Similarity)
	}
}
