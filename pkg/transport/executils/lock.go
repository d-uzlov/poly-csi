package executils

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/transport"
)

func (ec *ExecCommands) CreateLock(ctx context.Context, lockFile string, maxDuration time.Duration) (func(ctx context.Context) error, error) {
	lockId := uuid.NewString()

	res, err := ec.tr.RunCommand(ctx, []string{
		"flock",
		lockFile,
		"sh",
		"-eu",
		"-c",
		lockScript,
		lockId,
		lockFile,
		fmt.Sprintf("%d", maxDuration/time.Nanosecond),
	})
	if err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	if res.ExitCode != 0 {
		logs.FromCtx(ctx).Error("acquire lock failed", "stdout", string(res.Stdout), "stderr", string(res.Stderr), "exit-code", res.ExitCode)
		return nil, fmt.Errorf("acquire lock: %w", transport.FormatRunError(res))
	}
	return func(ctx context.Context) error {
		return ec.ReleaseLock(ctx, lockFile, lockId)
	}, nil
}

func (ec *ExecCommands) ReleaseLock(ctx context.Context, lockFile string, lockId string) error {
	res, err := ec.tr.RunCommand(ctx, []string{
		"flock",
		lockFile,
		"sh",
		"-eu",
		"-c",
		unlockScript,
		lockId,
		lockFile,
	})
	if err != nil {
		return fmt.Errorf("release lock: %w", err)
	}
	if res.ExitCode != 0 {
		logs.FromCtx(ctx).Error("release lock failed", "stdout", string(res.Stdout), "stderr", string(res.Stderr), "exit-code", res.ExitCode)
		return fmt.Errorf("release lock: %w", transport.FormatRunError(res))
	}
	return nil
}

var lockScript = `
# parameter index starts from 0 because we are running this as literal commands, so $0 can not be a file name
want_lock_id=$0
file=$1
timeout_nanoseconds=$2

now_timestamp=$(date +%s%N)

read -r header rest < "$file" || true

if [ "$header" = "" ]; then
	header=poly_csi_lock_version_1
elif [ "$header" = "poly_csi_lock_version_1" ]; then
	read -r header lock_id start_timestamp end_timestamp < "$file" || exit 20
	if [ "$lock_id" = "" -o "$start_timestamp" = "" -o "$end_timestamp" = "" ]; then
		echo "could not parse lock file as values: header='$header' lock_id='$lock_id' start_timestamp='$start_timestamp' end_timestamp='$end_timestamp'"
		exit 11
	fi
	if [ ! "$lock_id" = "$want_lock_id" ]; then
		if [ "$now_timestamp" -lt "$end_timestamp" ]; then
			echo "existing lock id is different: lock_id='$lock_id' != want_lock_id='$want_lock_id'"
			exit 12
		fi
		echo "existing lock has expired"
	else
		echo "lock has the same id, will renew it"
	fi
else
	echo "could not parse lock header"
	exit 10
fi

start_timestamp=$now_timestamp
end_timestamp=$((start_timestamp + timeout_nanoseconds))
echo "creating new lock file"
echo "poly_csi_lock_version_1 $want_lock_id $start_timestamp $end_timestamp" > "$file"
`

var unlockScript = `
want_lock_id=$0
file=$1

now_timestamp=$(date +%s%N)

read -r header rest < "$file" || true

if [ "$header" = "" ]; then
	echo "expected locked state but file is not locked"
	exit 30
elif [ "$header" = "poly_csi_lock_version_1" ]; then
	read -r header lock_id start_timestamp end_timestamp < "$file" || exit 20
	if [ "$lock_id" = "" -o "$start_timestamp" = "" -o "$end_timestamp" = "" ]; then
		echo "could not parse lock file as values: header='$header' lock_id='$lock_id' start_timestamp='$start_timestamp' end_timestamp='$end_timestamp'"
		exit 11
	fi
	if [ ! "$lock_id" = "$want_lock_id" ]; then
		echo "file is locked with different lock id: lock_id='$lock_id'"
		exit 31
	fi
else
	echo "could not parse lock header"
	exit 10
fi

echo > "$file"
`
