package startup

func StartInit() {
	ReadConfig()
	SetLog()
	ConnectDb()
	MigrateDb()
}
