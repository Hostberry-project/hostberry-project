package metrics

import "sync/atomic"

// Contadores de peticiones HTTP por clase de estado (usados por middleware y health).
var (
	HTTPRequests2xx uint64
	HTTPRequests4xx uint64
	HTTPRequests5xx uint64
)

// Add2xx incrementa el contador de respuestas 2xx.
func Add2xx() { atomic.AddUint64(&HTTPRequests2xx, 1) }

// Add4xx incrementa el contador de respuestas 4xx.
func Add4xx() { atomic.AddUint64(&HTTPRequests4xx, 1) }

// Add5xx incrementa el contador de respuestas 5xx.
func Add5xx() { atomic.AddUint64(&HTTPRequests5xx, 1) }

// Load2xx devuelve el valor actual del contador 2xx.
func Load2xx() uint64 { return atomic.LoadUint64(&HTTPRequests2xx) }

// Load4xx devuelve el valor actual del contador 4xx.
func Load4xx() uint64 { return atomic.LoadUint64(&HTTPRequests4xx) }

// Load5xx devuelve el valor actual del contador 5xx.
func Load5xx() uint64 { return atomic.LoadUint64(&HTTPRequests5xx) }
