import React from 'react';
import QRCode from 'qrcode.react';

import './App.css';


class Permission extends React.Component {
    constructor(props) {
        super(props);
        this.state = {
            data: ""
        };
        console.log("make a Permission item")
        this.sse = new EventSource(`/api/request/${props.pid}`);
        this.sse.onerror = e => {
            console.log(e)
            console.log(this.sse.readyState)
        }
       
    }

    componentDidMount() {
        console.log('Did mount')
    
        this.sse.addEventListener('Permission', e=> {
            console.log(`Permission ${e.data}`)
            let parsed = JSON.parse(e.data);
            if (parsed && parsed.done) {
                e.target.close()
                this.props.done(true)
                return
            }
            if (parsed && !parsed.done && parsed.msg.length=== 0 ){
                e.target.close()
                this.props.done(false)
                return
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
            <div className="QRcode">
                <QRCode value={this.state.data.msg} size={500}/>
            </div>
        );
    }
}
export default Permission;
