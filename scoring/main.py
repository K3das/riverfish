import os
import time
import confluent_kafka
import confluent_kafka.admin
import sys
from multiprocessing import Process, Queue

import predict
import scoring
import logging
import proto.pipeline.ingest_pb2
import proto.pipeline.output_pb2

NUM_WORKERS=2

logging.basicConfig(level=logging.INFO)

consumer = confluent_kafka.Consumer({
    "bootstrap.servers": os.getenv("PANDA"),
    "client.id": "scoring-consumer",
    "group.id": "scoring",
    "auto.offset.reset": "earliest"
})

admin = confluent_kafka.admin.AdminClient({
    'bootstrap.servers': os.getenv("PANDA")
})

admin.create_topics([confluent_kafka.admin.NewTopic(
    os.getenv("OUTPUT_TOPIC"),
    1,
    config={
        "retention.ms": str(1000 * 60 * 10)
    }
)])


def do_score(domain_scorer, predictor, producer, key, pb_data):
    # TODO: Somehow normalize scoring, idfk

    cert = proto.pipeline.ingest_pb2.CertificateLog()
    cert.ParseFromString(pb_data)
    alg_score = domain_scorer.score_domain(cert.subject)
    if alg_score < 70:
        return

    ml_score = predictor.predict(cert.subject)[0][0]
    score = alg_score + (0.5 - ml_score) * 100

    if score < 80:
        return
    scoring.recent_domains.pop(0)
    scoring.recent_domains.append(cert.subject)
    if cert.subject[:2] == ".*":
        print("wtf stars!?")
    print(f"sus: {cert.subject} ({score})")

    output = proto.pipeline.output_pb2.ScoredCertificateLog()
    output.subject = cert.subject
    output.log_url = cert.log_url
    output.index = cert.index

    output.total_score = score
    output.alg_score = alg_score
    output.ml_score = ml_score

    producer.produce(os.getenv("OUTPUT_TOPIC"), key=key, value=output.SerializeToString())

class Worker(Process):
    def __init__(self, queue):
        super(Worker, self).__init__()
        self.queue = queue

    def run(self):
        predictor = predict.Predictor()
        predictor.load()

        domain_scorer = scoring.DomainScorer()
        domain_scorer.load()

        producer = confluent_kafka.Producer({
            "bootstrap.servers": os.getenv("PANDA"),
            "client.id": "scoring-producer",
            "linger.ms": 500,
        })

        for data in iter(self.queue.get, None):
            try:
                do_score(domain_scorer, predictor, producer, *data)
            except Exception as e:
                logging.error('processing error at %s', 'division', exc_info=e)



local_queue = Queue(maxsize=NUM_WORKERS*2)
for i in range(NUM_WORKERS): 
    Worker(local_queue).start()
    time.sleep(0.005)

try:
    consumer.subscribe([os.getenv("INGEST_TOPIC")])
    logging.info("hai - subscribed")

    while True:
        msg = consumer.poll()
        if msg is None: continue

        if msg.error():
            if msg.error().code() == confluent_kafka.KafkaError._PARTITION_EOF:
                sys.stderr.write('%% %s [%d] reached end at offset %d\n' %
                                    (msg.topic(), msg.partition(), msg.offset()))
            elif msg.error():
                raise confluent_kafka.KafkaException(msg.error())
        else:
            local_queue.put((msg.key().decode("utf-8"), msg.value()), block=True)
finally:
    consumer.close()