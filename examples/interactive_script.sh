#!/bin/sh
printf "Enter username: "
read user
printf "Enter email: "
read email
printf "Confirm? [y/n] "
read confirm
echo "Created user: ${user} <${email}> (confirmed: ${confirm})"
