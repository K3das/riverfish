import re
import uuid
import tldextract
import utils
import os
import yaml
import confusables
import Levenshtein

suspicious_yaml = os.path.dirname(
    os.path.realpath(__file__)) + '/suspicious.yaml'

external_yaml = os.path.dirname(os.path.realpath(__file__)) + '/external.yaml'

with open(suspicious_yaml, 'r') as f:
    suspicious = yaml.safe_load(f)

with open(external_yaml, 'r') as f:
    external = yaml.safe_load(f)

if external['override_suspicious.yaml'] is True:
    suspicious = external
else:
    if external['keywords'] is not None:
        suspicious['keywords'].update(external['keywords'])

    if external['tlds'] is not None:
        suspicious['tlds'].update(external['tlds'])

recent_domains = [""] * 50



class DomainScorer:
    def __init__(self):
        self.loaded = False
        self.pid = os.getpid()
    def load(self):
        self.extract = tldextract.TLDExtract(cache_dir=f"/tmp/{str(uuid.uuid4())}")
        self.loaded = True
        print(f"loaded scorer in pid {self.pid}")
        
    def score_domain(self, domain):
        if not self.loaded:
            self.load()

        score = 0
        if ".git." in domain or ".gitlab." in domain:
            return 0

        if domain.startswith('*.'):
            domain = domain[2:]

        domain_data = self.extract(domain)
        split_subdomain = domain_data.subdomain.split(".")
        if len(split_subdomain) > 5:
            score -= 10
        if f".{domain_data.suffix}" in suspicious['tlds']:
            score += 20

        if domain_data.registered_domain in [
                "tribe.so", "amazonaws.com", "azure.com", "aws.dev", "mcas.ms",
                "cas.ms", "appdomain.cloud", "windows.net", "hcp.to",
                "microsoftpki.net", "microsoft.com"
        ]:
            return 0

        if split_subdomain[0] in ["gitlab", "git", "test", "*"]:
            score -= 50

        if domain.startswith("*."):
            print("wtfff " + domain)

        for d in recent_domains:
            if domain_data.registered_domain in d:
                score -= 10

        if "staging" in split_subdomain:
            score -= 30

        # Removing TLD to catch inner TLD in subdomain (ie. paypal.com.domain.com)
        domain = '.'.join([domain_data.subdomain, domain_data.domain])

        # Higer entropy is kind of suspicious
        score += int(round(utils.entropy(domain) * 10))

        # Remove lookalike characters using list from http://www.unicode.org/reports/tr39
        domain = confusables.unconfuse(domain)

        words_in_domain = re.split("\W+", domain)

        # ie. detect fake .com (ie. *.com-account-management.info)
        if words_in_domain[0] in ['com', 'net', 'org']:
            score += 10

        # Testing keywords
        for word in suspicious['keywords']:
            if word in domain:
                score += suspicious['keywords'][word]

        # Testing Levenshtein distance for strong keywords (>= 70 points) (ie. paypol)
        for key in [k for (k, s) in suspicious['keywords'].items() if s >= 70]:
            # Removing too generic keywords (ie. mail.domain.com)
            for word in [
                    w for w in words_in_domain
                    if w not in ['email', 'mail', 'cloud']
            ]:
                if Levenshtein.distance(str(word), str(key)) == 1:
                    score += 70

        # Lots of '-' (ie. www.paypal-datacenter.com-acccount-alert.com)
        if 'xn--' not in domain and domain.count('-') >= 4:
            score += domain.count('-') * 3

        # Deeply nested subdomains (ie. www.paypal.com.security.accountupdate.gq)
        if domain.count('.') >= 3:
            score += domain.count('.') * 3

        return score