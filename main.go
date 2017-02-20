package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"strconv"
	"./rh_enum"
	"./rh_proc"
)

type Client struct {
	conn           net.Conn
	srv            *Server
	id             int
	id_partner     int
	size_w         int32
	size_h         int32
	len_x          int32
	byte_per_pixel int32
	index_update   bool
	// буферы данных
	buf_image_data       []byte
	buf_image_data_send  []byte
	buf_image_data_unzip []byte
	buf_index_image_data []bool
}

type Server struct {
	clients     map[int]*Client
	address     string
	generate_id chan bool
}

// Creates new tcp server instance
func NewServer(address string) *Server {

	fmt.Println("Creating server with address", address)
	srv := &Server{
		address: address,
	}
	srv.clients = make(map[int]*Client)
	srv.generate_id = make(chan bool, 1)
	return srv
}

func (s *Server) Start() {

	fmt.Println("Starting listen server")
	listener, err := net.Listen("tcp", s.address)

	if err != nil {
		log.Fatal("Error starting TCP server.")
		fmt.Println("Error starting TCP server.")
	}
	defer listener.Close()

	for {
		conn, _ := listener.Accept()
		fmt.Println("Connect:" + conn.RemoteAddr().String())
		client := &Client{conn: conn, srv: s, id: 0, id_partner: 0}
		go client.handleRequest()
	}
}

func (client *Client) handleRequest() {

	srv := client.srv
	exit_from_cicle := false

	for {

		if exit_from_cicle == true {
			break
		}
		command, err := rh_proc.ReadCommand(client.conn)

		if err != nil {
			client.conn.Close()
			break
		}

		data_read, len, err2 := rh_proc.ReadData(client.conn)

		if err2 != nil {
			client.conn.Close()
			break
		}

		// error read
		if len == -1 {
			client.conn.Close()
		}
		switch rh_enum.Command(command) {
		// запрос получение идентификатора
		case rh_enum.Get_id:

			// генерируем id
			srv.generate_id <- true
			client.id = srv.GetID()
			if client.id > 1 {
				srv.AddClient(client)
			}
			<-srv.generate_id
			// выходим, если не получилось сгенерировать id
			if client.id < 1 {
				break
			}

			// id получен, отсылаем его обратно
			data := rh_proc.GetSimpleData_Int(rh_enum.Set_id, client.id)
			err := SendData(client, data)
			if err != nil {
				break
			}
			// запрос на подключение
		case rh_enum.Get_connect:

			mas := rh_proc.Proc_GetConnection(data_read)
			client_partner := srv.GetClient(int(mas[0]))

			// клиент не найден
			if client_partner == nil {
				data := rh_proc.GetSimpleData_Int(rh_enum.Get_connect, int(rh_enum.Connection_not_found))
				err := SendData(client, data)
				if err != nil {
					break
				}
				// если уже подключены, то тоже отказываем в соединении
			} else if client.id_partner > 0 || client_partner.id_partner > 0 {
				data := rh_proc.GetSimpleData_Int(rh_enum.Get_connect, int(rh_enum.Connection_not_found))
				err := SendData(client, data)
				if err != nil {
					break
				}
			} else {
				// связываем клиентов
				client.id_partner = client_partner.id
				client_partner.id_partner = client.id

				// посылаем приглашение
				mas_send := make([]int32, 3)
				mas_send[0] = int32(rh_enum.Connection_ok)
				mas_send[1] = int32(client.id)
				mas_send[2] = mas[1]

				data := rh_proc.GetSimpleData_Int32Mas(rh_enum.Get_connect, mas_send)
				err := SendData(client_partner, data)
				if err != nil {
					break
				}
			}
		// Установка соединения
		case rh_enum.Set_connect:

			client_partner := srv.GetClient(client.id_partner)

			if client_partner == nil {
				// отправим отказ
				client.StopMessage()
				break
			}

			if client.id_partner <= 0 {
				log.Fatal("Client id " + string(client.id) + ". Установка соединения с не найденным партнером")
				break
			}
			mas := rh_proc.Proc_SetConnection(data_read)

			// Отказ подключения
			if rh_enum.ResultConnection(mas[0]) == rh_enum.Connection_negative {
				// убираем привязку
				client.id_partner = 0
				// отсылаем информация об отключении
				data := rh_proc.GetSimpleData_Int(rh_enum.Set_connect, int(rh_enum.Connection_negative))
				err := SendData(client_partner, data)
				if err != nil {
					break
				}
			} else if rh_enum.ResultConnection(mas[0]) == rh_enum.Connection_ok {
				// отсылаем информацию партнеру, что все прошло удачно
				data := rh_proc.GetSimpleData_Int(rh_enum.Set_connect, int(rh_enum.Connection_ok))
				err := SendData(client_partner, data)
				if err != nil {
					break
				}

			} else {
				log.Fatal("Client id " + string(client.id) + ". Не найден вариант ответа: " + string(mas[0]))
				break
			}

		// Передача параметров картинки, создаем соответствующие буферы
		case rh_enum.Set_size:

			client_partner := srv.GetClient(client.id_partner)
			// отправим отказ, если по каким-то причинам нет такого пратнера
			if client_partner == nil {
				client.StopMessage()
				break
			}

			// инициализируем соответствующие структуры
			mas := rh_proc.Proc_SetSize(data_read)
			client.InitData(mas)

			// пересылаем данные
			data := rh_proc.GetSimpleData_ByteMas(rh_enum.Set_size, data_read)
			err := SendData(client_partner, data)
			if err != nil {
				break
			}

		// получение новой картинки от клиента
		case rh_enum.Set_image:
			// обрабатываем картинку
			//client.SetImageData(&data_read)
			// подготавливаем и отправляем данные партнеру
			//client.SendImageData()
			client.SendImageDataBuf(&data_read)
		// запрос получения картинки
		case rh_enum.Get_image:
			// подготовка и отправка данных
			client.SendImageData()

		case rh_enum.Set_stop:
			client_partner := srv.GetClient(client.id_partner)
			// отправим отказ, если по каким-то причинам нет такого пратнера
			if client_partner == nil {
				client.StopMessage()
				break
			}
			// пересылаем данные
			data := rh_proc.GetSimpleData_ByteMas(rh_enum.Set_stop, data_read)
			err := SendData(client_partner, data)
			if err != nil {
				break
			} else {
				client.id_partner = 0
				client_partner.id_partner = 0
			}

		case rh_enum.Echo:

			// пересылаем данные
			data := rh_proc.GetSimpleData_Int(rh_enum.Echo_ok, 0)
			err := SendData(client, data)
			if err != nil {
				break
			}
		case rh_enum.Ping:

			// пересылаем данные
			data := rh_proc.GetSimpleData_Int(rh_enum.Ping, 0)
			err := SendData(client, data)
			if err != nil {
				break
			}

		case rh_enum.File_command, rh_enum.Set_clipboard_data, rh_enum.Set_mouse, rh_enum.Get_size:
			client_partner := srv.GetClient(client.id_partner)
			// отправим отказ, если по каким-то причинам нет такого пратнера
			if client_partner == nil {
				client.StopMessage()
				break
			}

			// пересылаем данные
			data := rh_proc.GetSimpleData_ByteMas(rh_enum.Command(command), data_read)
			err := SendData(client_partner, data)
			if err != nil {
				break
			}
		default:
			exit_from_cicle = true
			break
		}
	}
	client.conn.Close()

	// удаляем клиента из сервера
	srv.generate_id <- true
	srv.DeleteClient(client.id)
	<-srv.generate_id
}

func (c *Client) InitData(mas []int32) error {

	// порядок данных mas: width, heigth, len, size
	c.size_w = mas[0]
	c.size_h = mas[1]
	c.len_x = mas[2]
	c.byte_per_pixel = mas[3]
	c.index_update = false
	// выделяем память
	//w_count := c.size_w / c.len_x

	//c.buf_image_data = make([]byte, c.size_w*c.size_h*c.byte_per_pixel)
	//c.buf_image_data_send = make([]byte, c.size_w*c.size_h+c.size_w*c.size_h*c.byte_per_pixel)
	//c.buf_image_data_unzip = make([]byte, 4*w_count*c.size_h+c.size_w*c.size_h*c.byte_per_pixel)
	//c.buf_index_image_data = make([]bool, w_count*c.size_h)

	return nil
}

func (c *Client) SetImageData(data *[]byte) error {

	// данные приходят заархивированными в zip
	b := bytes.NewReader(*data)
	z, err := gzip.NewReader(b)
	if err != nil {
		fmt.Println("error gzip:", err)
		return err
	}
	defer z.Close()

	buf, err := ioutil.ReadAll(z)

	//fmt.Println("Данные прочитаны, их длина:", len(buf))

	if err != nil {
		return err
	}

	if len(buf) > 0 {

		offset := copy(c.buf_image_data_unzip, buf)
		index_j := c.len_x * c.byte_per_pixel
		offset_local := 0

		var buf_index []byte
		var index int32

		for offset_local < offset {

			buf_index = c.buf_image_data_unzip[offset_local : offset_local+4]
			index = readInt32(buf_index)
			offset_local += 4

			copy(c.buf_image_data[index:index+index_j], c.buf_image_data_unzip[offset_local:offset_local+int(index_j)])

			offset_local += int(index_j)
			c.buf_index_image_data[index/index_j] = false
		}
		// выставляем признак появления новых данных
		if offset_local > 0 {
			c.index_update = true
		}
	}

	return nil
}

func (c *Client) SendImageData() error {

	client_partner := c.srv.GetClient(c.id_partner)
	// отправим отказ, если по каким-то причинам нет такого пратнера
	if client_partner == nil {
		c.StopMessage()
		err := errors.New("SendImageData(). Партнер не определен")
		return err
	}

	offset, err := c.PrepareImageData()
	if err != nil {
		return err
	}

	if offset == 0 {
		data := rh_proc.GetSimpleData_Int(rh_enum.Get_image, 0)
		err := SendData(client_partner, data)
		if err != nil {
			return err
		}
	} else {
		data := rh_proc.GetSimpleData_ByteMas(rh_enum.Get_image, c.buf_image_data_send[:offset])
		err := SendData(client_partner, data)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) SendImageDataBuf(data *[]byte) error {

	client_partner := c.srv.GetClient(c.id_partner)
	// отправим отказ, если по каким-то причинам нет такого пратнера
	if client_partner == nil {
		c.StopMessage()
		err := errors.New("SendImageData(). Партнер не определен")
		return err
	}

	if len(*data) == 0 {
		data := rh_proc.GetSimpleData_Int(rh_enum.Get_image, 0)
		err := SendData(client_partner, data)
		if err != nil {
			return err
		}
	} else {
		buf := rh_proc.GetSimpleData_ByteMas(rh_enum.Get_image, *data)
		err := SendData(client_partner, buf)
		if err != nil {
			return err
		}
	}
	return nil
}
func (c *Client) PrepareImageData() (int, error) {

	var index, index_j, offset int
	offset = 0
	// копируем нужные данные для отправки
	if c.index_update {
		index_j = int(c.len_x * c.byte_per_pixel)
		for i := 0; i < len(c.buf_index_image_data); i++ {
			if c.buf_index_image_data[i] == false {
				index = i * index_j
				b := Int32to4Byte(int32(index))
				copy(c.buf_image_data_send[offset:offset+4], b)
				offset += 4
				copy(c.buf_image_data_send[offset:offset+index_j], c.buf_image_data[index:index+index_j])
				offset += index_j
				c.buf_index_image_data[i] = true
			}
		}
		c.index_update = false
	}

	// архивируем данные для отправки и копируем в буфер для отправки
	if offset != 0 {

		var buf_in bytes.Buffer
		w := gzip.NewWriter(&buf_in)
		w.Write(c.buf_image_data_send[:offset])
		w.Close()

		b := buf_in.Bytes()
		offset = len(b)
		copy(c.buf_image_data_send[:offset], b)

	}

	return offset, nil
}

func readInt32(b []byte) int32 {
	//return int32(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24)
	return int32(binary.LittleEndian.Uint32(b))
}

func Int32to4Byte(val int32) []byte {

	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(val))
	return b
}

func (c *Client) StopMessage() error {
	c.id_partner = 0
	data := rh_proc.GetSimpleData_Int(rh_enum.Set_stop, 0)
	err := SendData(c, data)
	return err
}

func (s *Server) GetID() int {

	i := 0
	var val int

	for {
		val = rand.Intn(999)
		if val < 100 {
			val += 100
		}

		if i > 1000 {
			val = -1
			break
		}

		if _, ok := s.clients[val]; ok {
			i = i + 1
			continue
		} else {
			break
		}
	}

	return val
}

func (s *Server) DeleteClient(id int) {

	_, ok := s.clients[id]
	if ok {
		delete(s.clients, id)
	}
}

func (s *Server) GetClient(id int) *Client {

	element, ok := s.clients[id]
	if ok {
		return element
	} else {
		fmt.Println("не нашли клиента:" + strconv.Itoa(int(id)))
		return nil
	}
}

func SendData(c *Client, data []byte) error {

	_, err := c.conn.Write(data)
	return err
}

func (s *Server) AddClient(c *Client) {

	s.clients[c.id] = c

}

func main() {

	srv := NewServer(":45823")
	srv.Start()
}
