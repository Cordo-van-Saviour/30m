# 30m - Public Checkbox Application

30m is an example application designed to showcase 30 million public checkboxes that can be toggled in real-time. The project focuses on optimization and low-level bit manipulation, leveraging a bitset to efficiently store and manipulate the state of a large number of checkboxes. This repository contains both the backend written in Go and the frontend implemented with React.

## Features

- **Real-time Updates**: Utilizes WebSockets for real-time communication between the server and clients, ensuring that checkbox states are synchronized across all connected users.
- **Optimized Data Storage**: Employs bit manipulation techniques to manage checkbox states efficiently with a bitset, reducing memory usage considerably.
- **Run-Length Encoding**: Implements RLE (Run-Length Encoding) for compact storage of checkbox states when transmitting data to the client.
- **Image Rendering**: Converts the bitset into a WebP image format, allowing for easy visualization of the checkbox states.
  
## Technologies Used

- **Backend**: Go, gorilla/websocket, Redis, bits-and-blooms/bitset, chai2010/webp.
- **Frontend**: React, react-virtualized (for efficient rendering of large lists).
  
## Getting Started

### Prerequisites

- Go (1.18 or later)
- Node.js (for running the React frontend)
- Redis server (for storing the bitset)
- Installation of required Go packages (see next section)

### Installation

1. **Clone the Repository**:

   ```bash
   git clone https://github.com/yourusername/30m.git
   cd 30m
   ```

2. **Setup Backend**:

   - Navigate to the backend directory, and install dependencies:

   ```bash
   go mod tidy
   ```

   - Ensure Redis is running on your machine or set the `REDIS_URL` environment variable to your Redis service.
   - Build and run the server:

   ```bash
   go run main.go
   ```

   The server will start on `http://127.0.0.1:8080`.

3. **Setup Frontend**:

   - Navigate to the frontend directory:

   ```bash
   cd frontend
   ```

   - Install the necessary packages:

   ```bash
   npm install
   ```

   - Start the React development server:

   ```bash
   npm start
   ```

   The frontend should now be accessible at `http://localhost:3000`.

## Usage

1. Open your web browser and navigate to the frontend application.
2. You will see a grid of checkboxes. Click on any checkbox to toggle its state.
3. All changes will be reflected in real-time across all connected clients.

## Directory Structure

```
30m/
|── main.go                 # Go backend for managing bitsets and WebSocket connections
├── rle/                
│   ├── rle.go              # Run-Length Encoding implementation
│   └── ...
├── frontend/               # React frontend for displaying checkboxes
│   ├── src/
│   │   ├── App.js          # Main application component
│   │   ├── components/     # Reusable components
│   │   └── ...
├── go.mod                  # Go module file
└── package.json            # NPM package file for React app
```

## Optimization Strategies

- **Bit Manipulation**: The application utilizes a `bitset` to store checkbox states efficiently, allowing for compact data representation.
- **Run-Length Encoding**: RLE is applied for reducing the data size during transmission, optimizing network bandwidth usage.
- **Virtualized List**: The use of `react-virtualized` enhances rendering performance, allowing the application to handle millions of checkboxes smoothly.

## Contributing

Contributions are welcome! Feel free to fork the repository, create a new branch, and submit a pull request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

For any questions or issues, please feel free to create an issue on the repository. Happy coding!