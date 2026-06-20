package tor

// ResetTorState revierte la configuración de Tor aplicada durante el asistente: desactiva el
// redireccionamiento iptables y detiene/inhabilita el servicio. Se usa al reabrir el asistente
// sin finalizarlo, para volver a un estado limpio.
func ResetTorState(user string) {
	disableTorIptables(user)
	disableTor(user)
}
