package rh_proc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"test1/rh_enum"
)

func Read4b_with_error(conn net.Conn, buf []byte) error {

	//buf := make([]byte, 4)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		//fmt.Println("Error read command:", err)
		return err
	}
	return nil
}

func Read4b(conn net.Conn) (int, error) {

	// 4b - command
	buf := make([]byte, 4)
	_, err := io.ReadFull(conn, buf)
	//err := Read4b_with_error(conn, buf)
	if err != nil {
		//fmt.Println("Error read command:", err)
		return -1, err
	}

	return b4_to_int(buf), nil
}

func b4_to_int(data []byte) int {

	var val int32
	buf := bytes.NewBuffer(data)
	binary.Read(buf, binary.LittleEndian, &val)
	return int(val)
}

func ReadCommand(conn net.Conn) (int, error) {

	command, err := Read4b(conn)
	if err != nil {
		return command, err
	}
	//fmt.Println("Read command:", strconv.Itoa(command))
	return command, nil
}

func ReadData(conn net.Conn) ([]byte, int, error) {

	len, err := Read4b(conn)
	var data []byte

	if err != nil {
		return data, len, err
	}
	if len == 0 {
		return data, 0, nil
	}

	if len > 1024*1024*50 || len < 0 {
		err := errors.New("ReadData. Более 50 Мб выделено")
		return data, -1, err
	}

	buf := make([]byte, len)

	len, err = io.ReadFull(conn, buf)

	if err != nil {
		//fmt.Println("Error read data:", err)
		len = -1
		return data, len, err
	}

	return buf, len, nil
}

func GetSimpleData_Int(cmd rh_enum.Command, val int) []byte {

	buf := new(bytes.Buffer)

	binary.Write(buf, binary.LittleEndian, int32(cmd))
	binary.Write(buf, binary.LittleEndian, int32(4))
	binary.Write(buf, binary.LittleEndian, int32(val))

	return buf.Bytes()
}

func GetSimpleData_Int32Mas(cmd rh_enum.Command, mas []int32) []byte {

	buf := new(bytes.Buffer)

	// записываем заголовок
	binary.Write(buf, binary.LittleEndian, int32(cmd))
	binary.Write(buf, binary.LittleEndian, int32(len(mas)*4))

	// записываем данные
	for _, val := range mas {
		binary.Write(buf, binary.LittleEndian, val)
		//fmt.Println("GetSimpleData_Int32Mas: ", val)
	}

	return buf.Bytes()
}

func GetSimpleData_ByteMas(cmd rh_enum.Command, mas []byte) []byte {

	buf := new(bytes.Buffer)

	// записываем заголовок
	binary.Write(buf, binary.LittleEndian, int32(cmd))
	binary.Write(buf, binary.LittleEndian, int32(len(mas)))

	// записываем данные
	binary.Write(buf, binary.LittleEndian, mas)

	b := buf.Bytes()
	//fmt.Println("len data cmd"+strconv.Itoa(int(cmd)), strconv.Itoa(len(b)))
	return b
}
func Proc_GetConnection(data []byte) []int32 {

	var val int32
	mas := make([]int32, 2)

	buf := bytes.NewBuffer(data)
	binary.Read(buf, binary.LittleEndian, &val)
	mas[0] = val // id_client
	binary.Read(buf, binary.LittleEndian, &val)
	mas[1] = val // byte_per_pixel
	//fmt.Println("Proc_GetConnection [0,1]:"+strconv.Itoa(int(mas[0])), strconv.Itoa(int(mas[1])))
	return mas
}

func Proc_SetConnection(data []byte) []int32 {

	var val int32
	mas := make([]int32, 1)

	buf := bytes.NewBuffer(data)
	binary.Read(buf, binary.LittleEndian, &val)
	mas[0] = val // answer
	return mas
}

func Proc_SetSize(data []byte) []int32 {
	// размер входящих данных 4*4
	// порядок данных: width, heigth, len, size
	mas := GetInt32ArrayFromByteArray(data, 4)
	return mas
}

func GetInt32ArrayFromByteArray(data []byte, len int) []int32 {

	var val int32
	mas := make([]int32, len)

	buf := bytes.NewBuffer(data)
	for i := 0; i < len; i++ {
		binary.Read(buf, binary.LittleEndian, &val)
		mas[i] = val //
	}

	return mas
}
