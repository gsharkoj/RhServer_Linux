package rh_enum

type Command int
type ResultConnection int

const (
	Set_size           Command = 100
	Set_image          Command = 101
	Read_image         Command = 102
	Image_update       Command = 103
	Set_mouse          Command = 104
	Mouse_update       Command = 105
	Echo               Command = 106
	Echo_ok            Command = 107
	Get_image          Command = 108
	Get_id             Command = 109
	Set_id             Command = 110
	Get_connect        Command = 111
	Set_connect        Command = 112
	Get_size           Command = 113
	Get_stop           Command = 114
	Set_stop           Command = 115
	Data_fail          Command = 116
	Key_mouse_update   Command = 117
	Set_clipboard_data Command = 118
	Set_image_size     Command = 119
	File_command       Command = 120
	Ping               Command = 121
)

const (
	Connection_ok        ResultConnection = 100
	Connection_negative  ResultConnection = 101
	Connection_not_found ResultConnection = 102
)
