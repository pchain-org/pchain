// SPDX-License-Identifier: GPL-3.0

pragma solidity >=0.4.0 <0.9.0;

interface APIs {
   function callKeccak256(string memory input) external returns (bytes32);
   function generateRandomness(uint256 seed) external returns (uint256);
   function numberAdd(uint256 num) external returns (uint256);
   function stringAdd(string memory str) external returns (string memory);
   function bytes32Add(bytes32 by) external returns (bytes32);
   function mixAdd(string memory str, uint256 num) external returns (string memory);
}

contract Invoker {

    APIs apis;
    uint256 public randomResult;
    bytes32 public k256Result;
    uint256 public numberAddResult;
    string  public stringAddResult;
    bytes32 public bytes32AddResult;
    string  public mixAddResult;

    constructor() {
        apis = APIs(0x0000000000000000000000000000000000000066);
    }

    function callKeccak256(string memory input) public returns (bytes32) {
        k256Result = apis.callKeccak256(input);
        return k256Result;
    }

    function getRandomNumber(uint256 seed) public returns (uint256) {
        randomResult = apis.generateRandomness(seed);
        return randomResult;
    }

    function numberAdd(uint256 num) public returns (uint256) {
        numberAddResult = apis.numberAdd(num);
        return numberAddResult;
    }

    function stringAdd(string memory str) external returns (string memory) {
        stringAddResult = apis.stringAdd(str);
        return stringAddResult;
    }

    function bytes32Add(bytes32 by) external returns (bytes32) {
        bytes32AddResult = apis.bytes32Add(by);
        return bytes32AddResult;
    }

    function mixAdd(string memory str, uint256 num) external returns (string memory) {
       mixAddResult = apis.mixAdd(str, num);
       return mixAddResult;
    }

}
