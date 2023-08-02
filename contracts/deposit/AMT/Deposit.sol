// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;


import "@openzeppelin/contracts/access/Ownable.sol";
import "./IDeposit.sol";


contract Deposit is Ownable, IDeposit {
    //    using Address for address payable;

    mapping(address => uint256) private deposits;
    mapping(address => uint64) private depositTime;
    uint64 private depositLockingTime;
    uint64 private fiftyDepositLimit;
    uint64 private oneHundredDepositLimit;
    uint64 private fiveHundredDepositLimit;
    //
    uint256 private allDeposits = 0;
    uint64 private fiftyDepositCount = 0;
    uint64 private oneHundredDepositCount = 0;
    uint64 private fiveHundredDepositCount = 0;

    uint256 constant private fiftyDeposit = 50 ether;
    uint256 constant private oneHundredDeposit = 100 ether;
    uint256 constant private fiveHundredDeposit = 500 ether;


    modifier onlyOperator() {
        require(msg.sender == owner(), "Caller is not Operator");
        _;
    }

    modifier onlyDeposit() {
        require(deposits[msg.sender] > 0, "Do not have deposit");
        _;
    }

    constructor (uint64 _depositLockingTime, uint64 _fiftyDepositLimit, uint64 _oneHundredDepositLimit, uint64 _fiveHundredDepositLimit) Ownable() {
        depositLockingTime = _depositLockingTime;
        fiftyDepositLimit = _fiftyDepositLimit;
        oneHundredDepositLimit = _oneHundredDepositLimit;
        fiveHundredDepositLimit = _fiveHundredDepositLimit;
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
        require(amount == oneHundredDeposit || amount == fiveHundredDeposit || amount == fiftyDeposit, "DepositContract: payee is not allowed to deposit");
        //
        if (amount == fiftyDeposit) {
            require(fiftyDepositLimit > fiftyDepositCount, "10 AMC Deposit Limit has been reached");
        }
        //
        if (amount == oneHundredDeposit) {
            require(oneHundredDepositLimit > oneHundredDepositCount , "100 AMC Deposit Limit has been reached");
        }
        //
        if (amount == fiveHundredDeposit) {
            require(fiveHundredDepositLimit > fiveHundredDepositCount, "500 AMC Deposit Limit has been reached");
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
        if (amount == fiftyDeposit) {
            fiftyDepositCount++;
        }
        //
        if (amount == oneHundredDeposit) {
            oneHundredDepositCount++;
        }
        //
        if (amount == fiveHundredDeposit) {
            fiveHundredDepositCount++;
        }

        emit DepositEvent(pubkey, amount, signature);
    }

    function withdrawalAllowed(uint64 timestamp) public view returns (bool) {
        require(deposits[msg.sender] > 0 && depositTime[msg.sender] > 0, "Do not have deposits");
        require(address(this).balance >= deposits[msg.sender], "Insufficient balance");
        require(depositTime[msg.sender] + depositLockingTime <= timestamp, "Deposit is locking");
        return true;
    }

    function withdraw() public payable virtual override onlyDeposit {

        require(withdrawalAllowed(uint64(block.timestamp)), "DepositContract: payee is not allowed to deposit");
        uint256 amount = deposits[msg.sender];
        //
        sendValue(payable(msg.sender), amount);
        delete deposits[msg.sender];
        delete depositTime[msg.sender];

        //
        if (amount == fiftyDeposit) {
            fiftyDepositCount--;
        }
        //
        if (amount == oneHundredDeposit) {
            oneHundredDepositCount--;
        }
        //
        if (amount == fiveHundredDeposit) {
            fiveHundredDepositCount--;
        }

        allDeposits -= amount;
        emit WithdrawnEvent(amount);
    }

    function addDepositLimit(uint256 amount, uint64 limit) public  virtual  onlyOperator {
        //
        if (amount == fiftyDeposit) {
            fiftyDepositLimit += limit;
        }
        //
        if (amount == oneHundredDeposit) {
            oneHundredDepositLimit += limit;
        }
        //
        if (amount == fiveHundredDeposit) {
            fiveHundredDepositLimit += limit;
        }
    }
    function getDepositLimit(uint256 amount) public view  returns(uint64) {
        //
        if (amount == fiftyDeposit) {
            return fiftyDepositLimit;
        }
        //
        if (amount == oneHundredDeposit) {
            return oneHundredDepositLimit;
        }
        //
        if (amount == fiveHundredDeposit) {
            return fiveHundredDepositLimit;
        }

        return 0;
    }

    function getDepositRemain() public view returns(uint64, uint64, uint64) {
        uint64 fiftyDepositRemain = fiftyDepositLimit - fiftyDepositCount;
        uint64 oneHundredDepositRemain = oneHundredDepositLimit - oneHundredDepositCount;
        uint64 fiveHundredDepositRemain = fiveHundredDepositLimit - fiveHundredDepositCount;
        return (fiftyDepositRemain, oneHundredDepositRemain, fiveHundredDepositRemain);
    }

    function sendValue(address payable recipient, uint256 amount) private {
        require(address(this).balance >= amount, "Insufficient balance");
        (bool success, ) = recipient.call{value: amount}("");
        require(success, "Unable to send value, recipient may have reverted");
    }

}


