FROM tensorflow/tensorflow

RUN addgroup --system appgroup && adduser appuser --system --ingroup appgroup --home /home/appuser
COPY --chown=appuser:appgroup . /opt/app
WORKDIR /opt/app
USER appuser

RUN python3 -m pip install -r requirements.txt

ENTRYPOINT ["python3", "-u" ,"main.py"]