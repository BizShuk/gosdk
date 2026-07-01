module.exports = {
    apps: [
        // agy-gosdk (planner)
        {
            name: "agy-gosdk-system",
            script: "agy",
            args: [
                "--add-dir",
                "/Users/shuk/projects/tmp/gosdk",
                "-p",
                "'run /system-planner for current workspace'"
            ],
            namespace: "planner",
            cwd: "/Users/shuk/projects/tmp/gosdk",
            instances: 1,
            cron: "40 0-9 * * *"
        }
    ]
};
