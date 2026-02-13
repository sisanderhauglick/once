package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUniqueName(t *testing.T) {
	ns := &Namespace{name: "test"}

	assert.Equal(t, "myapp", ns.UniqueName("myapp"))

	ns.AddApplication(ApplicationSettings{Name: "myapp"})
	assert.Equal(t, "myapp-1", ns.UniqueName("myapp"))

	ns.AddApplication(ApplicationSettings{Name: "myapp-1"})
	assert.Equal(t, "myapp-2", ns.UniqueName("myapp"))

	// Unrelated app doesn't affect the name
	assert.Equal(t, "other", ns.UniqueName("other"))
}
