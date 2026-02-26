package startup

func StartInit() {
	ReadConfig()
	SetLog()
	MigrateDb()
}
