import './App.css';
import Login from './Login';
import MainPage from './MainPage';
import { QueryClient, QueryClientProvider, useQuery } from 'react-query'
import React from 'react';


import { ReactQueryDevtools } from 'react-query/devtools';
const queryClient = new QueryClient()


function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ReactQueryDevtools initialIsOpen={false} />
      <AppCode />
    </QueryClientProvider>
  )
}


class AppCode extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      auth: ""
    };

  }
  componentDidMount() {
    this.getLoginStatus()
  }

  getLoginStatus() {
    fetch('/api/login').then((res) => res.json()).then(data => this.setState({ auth: data.status }))
  }

  render() {
    if (!this.state.auth) return (
      <div className="QRcode" >
        <Login done={ this.getLoginStatus.bind(this)} />
      </div>
    )
    return (
    <div>
      <MainPage/>
    </div>
    )
  }
};

// function AppCode() {
//   const { isLoading, error, data } = useQuery('login', async () =>
//     fetch('/api/login').then((res) => res.json()))

//   if (isLoading) return 'Loading...'
//   console.log(data, error)

//   if (!data.status) return (
//     <div className="App">
//       <Login done={}/>
//     </div>
//   )

//   return (
//     <div className="App">
//       {/* <header className="App-header">
//       </header> */}
//       {/* <Login qrdata="This is a test data" /> */}
//     </div>
//   );
// }

export default App;
