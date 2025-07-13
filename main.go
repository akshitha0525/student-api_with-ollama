package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/mux"
)

/////////////////////////
// 1. Define Student Struct
/////////////////////////

type Student struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

/////////////////////////
// 2. Global Variables (In-memory store + mutex)
/////////////////////////

var (
	students = make(map[int]Student)
	mutex    = &sync.Mutex{}
)

/////////////////////////
// 3. HANDLERS
/////////////////////////

// 🔵 Create Student
func createStudent(w http.ResponseWriter, r *http.Request) {
	var student Student
	err := json.NewDecoder(r.Body).Decode(&student)
	if err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	if student.Name == "" || student.Email == "" || student.Age <= 0 {
		http.Error(w, "Invalid student data", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	student.ID = len(students) + 1
	students[student.ID] = student

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(student)
}

// 🟢 Get All Students
func getStudents(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	defer mutex.Unlock()

	var list []Student
	for _, s := range students {
		list = append(list, s)
	}

	json.NewEncoder(w).Encode(list)
}

// 🟡 Get Student by ID
func getStudent(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		http.Error(w, "Invalid student ID", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	student, exists := students[id]
	mutex.Unlock()

	if !exists {
		http.Error(w, "Student not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(student)
}

// 🟠 Update Student by ID
func updateStudent(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		http.Error(w, "Invalid student ID", http.StatusBadRequest)
		return
	}

	var updated Student
	err = json.NewDecoder(r.Body).Decode(&updated)
	if err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	if updated.Name == "" || updated.Email == "" || updated.Age <= 0 {
		http.Error(w, "Invalid student data", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	_, exists := students[id]
	if !exists {
		http.Error(w, "Student not found", http.StatusNotFound)
		return
	}

	updated.ID = id
	students[id] = updated

	json.NewEncoder(w).Encode(updated)
}

// 🔴 Delete Student by ID
func deleteStudent(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		http.Error(w, "Invalid student ID", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	_, exists := students[id]
	if !exists {
		http.Error(w, "Student not found", http.StatusNotFound)
		return
	}

	delete(students, id)
	w.WriteHeader(http.StatusNoContent)
}

// 🧠 Generate Summary using Ollama
func getStudentSummary(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		http.Error(w, "Invalid student ID", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	student, exists := students[id]
	mutex.Unlock()

	if !exists {
		http.Error(w, "Student not found", http.StatusNotFound)
		return
	}

	// Prepare prompt for Ollama
	prompt := fmt.Sprintf("Provide a concise summary of the student profile:\nName: %s\nAge: %d\nEmail: %s\n",
		student.Name, student.Age, student.Email)

	// Prepare request body
	requestBody := map[string]interface{}{
		"model":  "llama3",
		"prompt": prompt,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		http.Error(w, "Failed to encode request", http.StatusInternalServerError)
		return
	}

	// Send POST request to Ollama API
	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, "Failed to call Ollama API", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	var fullResponse string

	for scanner.Scan() {
		var chunk struct {
			Response string `json:"response"`
			Done     bool   `json:"done"`
		}

		line := scanner.Text()
		err := json.Unmarshal([]byte(line), &chunk)
		if err != nil {
			http.Error(w, "Failed to parse Ollama response chunk", http.StatusInternalServerError)
			return
		}

		fullResponse += chunk.Response

		if chunk.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		http.Error(w, "Error reading Ollama response stream", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"summary": fullResponse})
}

/////////////////////////
// 4. MAIN Function
/////////////////////////

func main() {
	r := mux.NewRouter()

	// Register routes
	r.HandleFunc("/students", createStudent).Methods("POST")
	r.HandleFunc("/students", getStudents).Methods("GET")
	r.HandleFunc("/students/{id}", getStudent).Methods("GET")
	r.HandleFunc("/students/{id}", updateStudent).Methods("PUT")
	r.HandleFunc("/students/{id}", deleteStudent).Methods("DELETE")
	r.HandleFunc("/students/{id}/summary", getStudentSummary).Methods("GET")

	fmt.Println("Server running on http://localhost:8080")
	http.ListenAndServe(":8080", r)
}
