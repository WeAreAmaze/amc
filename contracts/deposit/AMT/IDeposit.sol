// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;


interface IDeposit{
    event DepositEvent(
        bytes pubkey,
        uint256 weiAmount,
        bytes signature
    );
    event WithdrawnEvent(uint256 weiAmount);

    function deposit(
        bytes calldata pubkey,
        bytes calldata signature
    ) external payable;

    function withdraw() external payable;
    function depositsOf(address payee) external view returns (uint256);
    function depositUnlockingTimestamp(address payee) external view returns (uint64);
    function getDepositCount() external view returns (uint256);
}