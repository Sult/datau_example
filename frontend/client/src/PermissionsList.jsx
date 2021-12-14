import React from 'react';
import Permission from './Permission';

function b64_to_utf8(str) {

    if (!str) {
        return str;
    }
    return decodeURIComponent(escape(atob(str)));
}

class PermissionsList extends React.Component {
    constructor(props) {
        super(props);
        this.state = {
            Permissions: {}
        };
    }

    mainMode(key, result) {
        
        this.state.Permissions[key].status = 0;
        if (result) {
            this.state.Permissions[key].status = 1;
        }
        this.setState({ Permissions: this.state.Permissions })
    }
    permissionMode(key) {
        this.state.Permissions[key].status = -1;
        this.setState({ Permissions: this.state.Permissions })
    }

    componentDidMount() {
        this.updatePerimssions()
    }

    async updatePerimssions() {
        await fetch('/api/dag').then((res) => res.json()).then(data => {
            let p = {}
            data.Didgraph.map((i) => p[i.Key] = i)
            this.setState({ Permissions: p })
        })
        fetch('/api/user/permissions').then((res) => res.json()).then(data => {
            Object.keys(data).map((k) => {
                console.log(this.state.Permissions)
                Object.assign(this.state.Permissions[k], data[k])
            })
            this.setState({ Permissions: this.state.Permissions })
        })
    }


    render() {

        let items = Object.entries(this.state.Permissions).map((item, k) => {
            let value = item[1].value
            if (item[1].Mime.indexOf("text/plain") !== -1) {
                value = b64_to_utf8(value);
            }
            let status = item[1].status;
            switch (status) {
                case 1:
                    status = "Have a local value"
                    break;
                case 2:
                    status = "Have a remote value"
                    break;
                case -1:
                    status = <Permission done={this.mainMode.bind(this, item[0])} pid={item[0]} />
                    break;
                case 0:
                default:
                    status = <button onClick={this.permissionMode.bind(this, item[0])} >Get Permission</button>
            }
            return (<div className='user_info' key={k}> {item[1].Description}: {value} <br/>{status}
            </div>)
        })

        return (
            <div>
                {items}
            </div>
        );
    }
}
export default PermissionsList;
