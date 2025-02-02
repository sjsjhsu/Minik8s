import os
import json
import func

from flask import Flask, request

app = Flask(__name__)


@app.route("/", methods=['POST'])
def work():
    try:
        params = json.loads(request.get_data())
        res = func.main(params)
    except Exception as e:
        res = {'error': str(e)}
    finally:
        return json.dumps(res)


if __name__ == '__main__':
    app.run(debug=True, host="0.0.0.0", port=int(os.environ.get('PORT', 8080)))
