import React, { useState, useCallback, useEffect, useMemo } from 'react';
import { List, AutoSizer } from 'react-virtualized';
import './App.css';

const rowCount = 1000000; // Number of checkboxes
const checkboxesPerRow = 30; // Number of checkboxes per row
const totalRows = Math.ceil(rowCount / checkboxesPerRow); // Total number of rows
const rowHeight = 40; // Height of each row in pixels

const App = () => {
  // const ws = new WebSocket('ws://localhost:8080/ws');
  // wrap ws in useMemo
  const ws = useMemo(() => new WebSocket('ws://localhost:8080/ws'), []);

  const [checkboxes, setCheckboxes] = useState(Array(rowCount).fill(false));

  // Handler to toggle checkbox state
  const handleCheckboxChange = useCallback((index) => {
    let newCheckboxes = [...checkboxes];
    let state = !newCheckboxes[index];
    setCheckboxes((prev) => {
      newCheckboxes[index] = state
      return newCheckboxes;
    });

    // Send the index and checked state to the server through WebSocket
    ws.send(`${index}:${state}`);

  }, [checkboxes, ws]);

  useEffect(() => {
    ws.onmessage = (event) => {
      try {
        const arr = JSON.parse(event.data);
        setCheckboxes(arr || []);
      } catch (error) {
        // if error is thrown while parsing, it's probably not a JSON string
        const res = event.data.split(":")
        setCheckboxes((prev) => {
          const newCheckboxes = [...prev];
          newCheckboxes[res[0]] = res[1] === 'true';
          return newCheckboxes;
        });
      }
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    ws.onclose = () => {
      console.log('WebSocket connection closed');
    }
  }, [checkboxes, ws]);


  const rowRenderer = ({ key, index, style }) => {
    const start = index * checkboxesPerRow;
    const end = Math.min(start + checkboxesPerRow, rowCount);
    const rowCheckboxes = [];

    for (let i = start; i < end; i++) {
      rowCheckboxes.push(
          <div key={i} className="checkbox-container">
            <input
                type="checkbox"
                id={`checkbox-${i}`}
                checked={checkboxes[i]}
                height={rowHeight}
                width={rowHeight}
                onChange={() => handleCheckboxChange(i)}
            />
          </div>
      );
    }


    return (
        <div key={key} style={style} className="row">
          {rowCheckboxes}
        </div>
    );
  };

  // create a wrapper inside to make it scrollable and inside
  return (
      <div id={'wrapper'} style={{ height: '100vh', width: "80%", margin: "2rem auto" }} >
        <AutoSizer>
          {({ height, width }) => (
              <List
                  width={width}
                  height={height}
                  rowCount={totalRows}
                  rowHeight={rowHeight}
                  rowRenderer={rowRenderer}
              />
          )}
        </AutoSizer>
      </div>
  );
};

export default App;