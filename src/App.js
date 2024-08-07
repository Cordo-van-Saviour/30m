import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { AutoSizer, List } from 'react-virtualized';
import './App.css';
import { Header } from "./components/Header/Header";
/* global BigInt */

const rowCount = 1000000; // Number of checkboxes
const checkboxesPerRow = 30; // Number of checkboxes per row
const totalRows = Math.ceil(rowCount / checkboxesPerRow); // Total number of rows
const rowHeight = 50; // Height of each row in pixels

const decodeRLE = (encoded) => {
  const bytes = atob(encoded);
  const uint8Array = new Uint8Array(bytes.length);
  for (let i = 0; i < bytes.length; i++) {
    uint8Array[i] = bytes.charCodeAt(i);
  }

  const result = [];
  let i = 0;
  while (i < uint8Array.length) {
    let value = 0n;
    let shift = 0n;
    let byte;
    do {
      byte = uint8Array[i++];
      value |= BigInt(byte & 0x7F) << shift;
      shift += 7n;
    } while (byte & 0x80);

    let run = 0n;
    shift = 0n;
    do {
      byte = uint8Array[i++];
      run |= BigInt(byte & 0x7F) << shift;
      shift += 7n;
    } while (byte & 0x80);

    // Convert the 64-bit integer to 8 bytes
    for (let j = 0; j < Number(run); j++) {
      for (let k = 0n; k < 64n; k += 8n) {
        result.push(Number((value >> k) & 0xFFn));
      }
    }
  }

  return new Uint8Array(result);
};


const App = () => {
  const ws = useMemo(() => new WebSocket(`ws://${window.location.hostname}/ws`), []);

  const [checkboxes, setCheckboxes] = useState(Array(rowCount).fill(false));
  useEffect(() => {
    fetch('/api')
        .then((response) => response.json())
        .then((data) => {
          const uint8Array = decodeRLE(data["bitsetRLE"]);
          const boolArray = bitsetToBooleanArray(uint8Array);

          setCheckboxes(boolArray);
        })
        .catch((error) => {
          console.error('Error:', error);
        })
  }, []);

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
        const res = event.data.split(":")
        setCheckboxes((prev) => {
          const newCheckboxes = [...prev];
          newCheckboxes[res[0]] = res[1] === 'true';
          return newCheckboxes;
        });
      } catch (error) {
        console.error('Error parsing message:', error);
      }
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    ws.onclose = () => {
      console.log('WebSocket connection closed');
    }
  }, [checkboxes, ws]);

  const rowRenderer = ({key, index, style}) => {
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
  
  const bitsetToBooleanArray = (uint8Array) => {
    const boolArray = [];
    uint8Array.forEach(byte => {
      for (let i = 0; i < 8; i++) {
        boolArray.push((byte & (1 << i)) !== 0);
      }
    });
    return boolArray;
  };


  // create a wrapper inside to make it scrollable and inside
  return (
      <>
        <Header>Header</Header>
        <div id={'wrapper'} style={{height: '100vh', width: "80%", margin: "2rem auto"}}>
          <AutoSizer>
            {({height, width}) => (
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
      </>
  );
};

export default App;
