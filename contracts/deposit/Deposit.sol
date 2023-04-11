// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;


import "@openzeppelin/contracts/access/Ownable.sol";
import "./IDeposit.sol";


contract Deposit is Ownable, IDeposit {
    //    using Address for address payable;

    mapping(address => uint256) private deposits;
    mapping(address => uint64) private depositTime;
    uint64 private depositLockingTime;
    uint64 private tenDepositLimit;
    uint256 private allDeposits = 0;

    uint256 constant private tenDeposit = 10 ether;
    uint256 constant private oneHundredDeposit = 100 ether;
    uint256 constant private fiveHundredDeposit = 500 ether;


    modifier onlyOperator() {
        require(deposits[msg.sender] > 0 || msg.sender == owner(), "Caller is not Operator");
        _;
    }

    constructor (uint64 _depositLockingTime, uint64 _tenDepositLimit) Ownable() {
        depositLockingTime = _depositLockingTime;
        tenDepositLimit = _tenDepositLimit;
    }

    function depositsOf(address payee) public view  override returns (uint256) {
        return deposits[payee];
    }

    function depositUnlockingTimestamp(address payee) public view  override  returns (uint64) {
        require(depositTime[payee] > 0, "DepositContract: do not have deposit");
        return depositTime[payee] + depositLockingTime;
    }

    function getDepositCount() public view override returns (uint256) {
        return allDeposits;
    }

    function depositAllowed(uint256 amount) public view returns (bool) {
        require(deposits[msg.sender] == 0, "DepositContract: you already deposited");
        require(amount == oneHundredDeposit || amount == fiveHundredDeposit || amount == tenDeposit, "DepositContract: payee is not allowed to deposit");
        //
        if (amount == tenDeposit) {
            require(tenDepositLimit > 0, "10 AMC Deposit Limit has been reached");
        }
        return true;
    }

    function deposit(bytes calldata pubkey, bytes calldata signature) public payable virtual override {

        uint256 amount = msg.value;
        require(depositAllowed(amount), "DepositContract: payee is not allowed to deposit");
        require(pubkey.length == 48, "DepositContract: invalid pubkey length");
        require(signature.length == 96, "DepositContract: invalid signature length");

        //
        deposits[msg.sender] = amount;
        depositTime[msg.sender] = uint64(block.timestamp);
        allDeposits += amount;
        //
        if (amount == tenDeposit) {
            tenDepositLimit--;
        }

        emit DepositEvent(pubkey, amount, signature);
    }

    function withdrawalAllowed(uint64 timestamp) public view returns (bool) {
        require(deposits[msg.sender] > 0 && depositTime[msg.sender] > 0, "Do not have deposits");
        require(address(this).balance >= deposits[msg.sender], "Insufficient balance");
        require(depositTime[msg.sender] + depositLockingTime <= timestamp, "Deposit is locking");
        return true;
    }

    function withdraw() public payable virtual override onlyOperator {

        require(withdrawalAllowed(uint64(block.timestamp)), "DepositContract: payee is not allowed to deposit");
        uint256 amount = deposits[msg.sender];
        //
        sendValue(payable(msg.sender), amount);
        delete deposits[msg.sender];
        delete depositTime[msg.sender];
        allDeposits -= amount;
        emit WithdrawnEvent(amount);
    }

    function sendValue(address payable recipient, uint256 amount) private {
        require(address(this).balance >= amount, "Insufficient balance");
        (bool success, ) = recipient.call{value: amount}("");
        require(success, "Unable to send value, recipient may have reverted");
    }

}


