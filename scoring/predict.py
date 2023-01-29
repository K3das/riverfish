# import tensorflow as tf
from tensorflow.keras import backend as K
from tensorflow.keras.preprocessing import sequence
from tensorflow.keras.preprocessing.text import Tokenizer
import json, os

os.environ['TF_CPP_MIN_LOG_LEVEL'] = '10'
os.environ['CUDA_VISIBLE_DEVICES'] = '-1'

def normalize_domains(indicators):
    if not isinstance(indicators, list):
        indicators = [indicators]

    return [i.lower() for i in indicators]


print("loading tokenizer")
with open(os.path.join("data/word-dict.json")) as F:
    txt = F.read()

txt = json.loads(txt)

tokenizer = Tokenizer(filters='\t\n', char_level=True, lower=True)
tokenizer.word_index = txt

class Predictor:
    def __init__(self):
        self.loaded = False
        self.pid = os.getpid()

    def load(self):
        import tensorflow as tf
        
        self.model = tf.keras.models.load_model("data/model.h5")
        self.model.load_weights("data/weights.h5")

        self.loaded = True
        print(f"loaded predictor in pid {self.pid}")

    def predict(self,i):
        if not self.loaded:
            self.load()
        i = normalize_domains(i)

        seq = tokenizer.texts_to_sequences(i)
        log_entry_processed = sequence.pad_sequences(seq, maxlen=255)

        p = self.model.predict(log_entry_processed, verbose="10")
        return p
