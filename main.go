package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

type Student struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

var (
	students = make(map[int]Student)
	mutex    = &sync.Mutex{}
)

func createStudent(w http.ResponseWriter, r *http.Request) {
	var student Student
	err := json.NewDecoder(r.Body).Decode(&student)
	if err != nil || student.Name == "" || student.Email == "" || student.Age <= 0 {
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

func getStudents(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	defer mutex.Unlock()

	var list []Student
	for _, s := range students {
		list = append(list, s)
	}

	json.NewEncoder(w).Encode(list)
}

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

func updateStudent(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		http.Error(w, "Invalid student ID", http.StatusBadRequest)
		return
	}

	var updated Student
	err = json.NewDecoder(r.Body).Decode(&updated)
	if err != nil || updated.Name == "" || updated.Email == "" || updated.Age <= 0 {
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

	prompt := fmt.Sprintf("Summarize this student profile: Name: %s, Age: %d, Email: %s", student.Name, student.Age, student.Email)

	requestBody := map[string]interface{}{
		"model":       "llama3",
		"prompt":      prompt,
		"temperature": 0.3,
		"top_p":       0.9,
		"max_tokens":  50,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		http.Error(w, "Failed to encode request", http.StatusInternalServerError)
		return
	}

	client := &http.Client{Timeout: 60 * time.Second}

	req, err := http.NewRequest("POST", "http://localhost:11434/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to call Ollama API: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("Ollama returned status %d", resp.StatusCode)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	var fullResponse strings.Builder

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

		fullResponse.WriteString(chunk.Response)

		if chunk.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		http.Error(w, "Error reading Ollama response stream", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"summary": fullResponse.String()})
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "✅ Student API is working! Visit /students or /students/{id}")
}

func main() {
	r := mux.NewRouter()

	// Root route
	r.HandleFunc("/", homeHandler).Methods("GET")

	// Student CRUD
	r.HandleFunc("/students", createStudent).Methods("POST")
	r.HandleFunc("/students", getStudents).Methods("GET")
	r.HandleFunc("/students/{id}", getStudent).Methods("GET")
	r.HandleFunc("/students/{id}", updateStudent).Methods("PUT")
	r.HandleFunc("/students/{id}", deleteStudent).Methods("DELETE")
	r.HandleFunc("/students/{id}/summary", getStudentSummary).Methods("GET")

	fmt.Println("Server running on http://localhost:8080")
	http.ListenAndServe(":8080", r)
}
