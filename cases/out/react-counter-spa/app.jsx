function Counter() {
  const [count, setCount] = React.useState(0);

  return (
    <div className="counter">
      <h1>Counter</h1>
      <p className="count">{count}</p>
      <div className="buttons">
        <button onClick={() => setCount(count - 1)}>Decrement</button>
        <button onClick={() => setCount(0)}>Reset</button>
        <button onClick={() => setCount(count + 1)}>Increment</button>
      </div>
    </div>
  );
}

const root = ReactDOM.createRoot(document.getElementById('root'));
root.render(<Counter />);
