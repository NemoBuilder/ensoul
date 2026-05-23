// SPDX-License-Identifier: MIT
// EnsoulCommunityToken — minimal ERC-20 deployed once per graduated Galaxy.
//
// Design notes:
//   - No upgradability, no admin keys after deploy.
//   - Entire `supply` is minted to `mintTo` in the constructor. For Galaxy
//     fair launches `mintTo` is the EnsoulFairLaunch contract address, so
//     depositors can `claim()` their pro-rata share trustlessly.
//   - Standalone (no OpenZeppelin import) to match the existing flattened
//     contract style in this repo.
pragma solidity ^0.8.20;

contract EnsoulCommunityToken {
    string public name;
    string public symbol;
    uint8  public constant decimals = 18;

    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);

    constructor(string memory _name, string memory _symbol, uint256 _supply, address _mintTo) {
        require(_mintTo != address(0), "ECT: mintTo zero");
        require(_supply > 0, "ECT: supply zero");
        name = _name;
        symbol = _symbol;
        totalSupply = _supply;
        balanceOf[_mintTo] = _supply;
        emit Transfer(address(0), _mintTo, _supply);
    }

    function transfer(address to, uint256 value) external returns (bool) {
        _transfer(msg.sender, to, value);
        return true;
    }

    function approve(address spender, uint256 value) external returns (bool) {
        allowance[msg.sender][spender] = value;
        emit Approval(msg.sender, spender, value);
        return true;
    }

    function transferFrom(address from, address to, uint256 value) external returns (bool) {
        uint256 a = allowance[from][msg.sender];
        require(a >= value, "ECT: allowance");
        if (a != type(uint256).max) {
            allowance[from][msg.sender] = a - value;
        }
        _transfer(from, to, value);
        return true;
    }

    function _transfer(address from, address to, uint256 value) internal {
        require(to != address(0), "ECT: to zero");
        uint256 b = balanceOf[from];
        require(b >= value, "ECT: balance");
        unchecked { balanceOf[from] = b - value; }
        balanceOf[to] += value;
        emit Transfer(from, to, value);
    }
}
