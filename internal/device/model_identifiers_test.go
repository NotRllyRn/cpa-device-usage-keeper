package device

import (
	"context"
	"testing"
)

func TestPricingIdentifiersLargeHistoryAndBounds(t *testing.T) {
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	id, _, err := s.Enroll("test")
	if err != nil {
		t.Fatal(err)
	}
	// Deliberately exceed Overview's snapshot cap without expensive aggregation.
	_, err = s.db.Exec(`WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<100001)
 INSERT INTO snapshots(device_id,agent,day,revision,digest,payload)
 SELECT ?, 'pi', CAST(x AS TEXT),1,'test','{"models":[{"model":"example"}]}' FROM n`, id)
	if err != nil {
		t.Fatal(err)
	}
	models, err := s.PricingModelIdentifiers(context.Background())
	if err != nil || len(models) != 1 || models[0] != "example" {
		t.Fatal(models, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.PricingModelIdentifiers(ctx); err == nil {
		t.Fatal("cancelled scan succeeded")
	}
	_, err = s.db.Exec(`WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<10001)
 INSERT INTO device_prices(model,payload) SELECT 'model-'||x,'{}' FROM n`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.PricingModelIdentifiers(context.Background()); err == nil {
		t.Fatal("unbounded identifiers")
	}
}
