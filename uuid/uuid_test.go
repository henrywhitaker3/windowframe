package uuid

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestItMakesV7satTime(t *testing.T) {
	now := time.Now().Add(-time.Minute * 5)

	one := Must(OrderedAt(now))
	two := Must(OrderedAt(now))

	t.Log(one.String())
	t.Log(two.String())

	t.Log(one[0:8])
	t.Log(two[0:8])

	for i := range 8 {
		if i < 7 {
			require.Equal(t, one[i], two[i], "index %d is not equal", i)
		}
		if i == 7 {
			require.Equal(t, int(one[i])+1, int(two[i]))
		}
	}

	require.NotEqual(t, one, two)
}

func FuzzNoCollisions(f *testing.F) {
	generated := map[UUID]struct{}{}

	f.Fuzz(func(t *testing.T, _ string) {
		id := Must(OrderedAt(time.Now()))
		if _, ok := generated[id]; ok {
			t.Fail()
		}
		generated[id] = struct{}{}
	})
}

func TestItOrderV7AtCorrectly(t *testing.T) {
	first := time.Now().Add(-time.Minute)
	last := time.Now()

	firstID := Must(OrderedAt(first))
	lastID := Must(OrderedAt(last))

	ids := []string{lastID.String(), firstID.String()}

	slices.Sort(ids)

	require.Equal(t, firstID.String(), ids[0])
	require.Equal(t, lastID.String(), ids[1])
}

func TestItMarshalsToYamlProperly(t *testing.T) {
	id := Must(New())

	expected := fmt.Sprintf("%s\n", id.String())

	by, err := yaml.Marshal(id)
	require.Nil(t, err)
	require.Equal(t, expected, string(by))
}

func TestItUnmarshalsFromYaml(t *testing.T) {
	plain := "01991c2b-13f8-7e45-9b2c-ed1230826bc6"

	var id UUID
	require.Nil(t, yaml.Unmarshal(fmt.Appendf(nil, "%s\n", plain), &id))
	require.Equal(t, plain, id.String())
}
