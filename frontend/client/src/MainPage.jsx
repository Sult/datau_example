import React from 'react';

import PermissionsList from './PermissionsList';

class Login extends React.Component {
    constructor(props) {
        super(props);
        this.state = {
            mode: 0
        };
    }

    componentDidMount() {
    }



    render() {
        return (
            <div>
                <PermissionsList/>
            </div>
        )
    }
}
export default Login;
