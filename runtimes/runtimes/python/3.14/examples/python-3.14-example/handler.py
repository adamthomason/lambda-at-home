import time


def handler(app, *args, **kwargs) -> None:
    limit = 20
    iteration = 1

    while iteration < limit:
        app.logger.info(f"This is coming from the handler ({iteration})")
        iteration += 1

    time.sleep(20)
