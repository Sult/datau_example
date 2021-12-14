import React from 'react';
import QRCode from 'qrcode.react';


class Login extends React.Component {
    constructor(props) {
        super(props);
        this.state = {
            data: ""
        };
        console.log("make a Login item")
        this.sse = new EventSource('/api/auth');
        this.sse.onerror = e => {
            console.log(e)
            console.log(this.sse.readyState)
        }
       
    }

    componentDidMount() {
        console.log('Did mount')
    
        this.sse.addEventListener('Login', e=> {
            console.log(`Login ${e.data}`)
            let parsed = JSON.parse(e.data);
            if (parsed && parsed.done) {
                e.target.close();
                this.props.done()
            }
            this.setState({data:parsed})
            
        })
    }
    componentWillUnmount() {
        this.sse.close();
    }

    render() {
        if (!this.state.data || this.state.data.done)  {
            return <div></div>
        }
        return (
            <div>
                <QRCode value={this.state.data.msg} width={70} height={70} size={500}/>
            </div>
        );
    }
}
export default Login;
