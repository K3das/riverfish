from asyncio import to_thread
from zipfile import is_zipfile
import pandas as pd
import os
import pathlib
import tldextract
from tqdm import tqdm


OUTPUT_PATH = "./output_data/"

pathlib.Path(OUTPUT_PATH).mkdir(exist_ok=True)


print("loading badurls")
bad_domains = pd.read_csv("mal.txt", header=None, names=["domain"])
to_drop = []
for i, domain in (progress := tqdm(bad_domains.iterrows(), total=len(bad_domains))):
    domain["domain"] = domain["domain"].split(":")[0]
    domain["domain"] = domain["domain"].replace("@", "")
    url_data = tldextract.extract(domain["domain"])
    bad_domains.at[i, "sld"] = url_data.registered_domain
    bad_domains.at[i, "is_subdomain"] = url_data.subdomain != ""

    if url_data.suffix == "":
        progress.set_description(domain["domain"])
        progress.total -= 1
        to_drop.append(i)
        continue
    
bad_domains.drop(bad_domains.index[to_drop], inplace=True)
print(bad_domains[:5])

common_bad_domains = bad_domains.loc[
    bad_domains["is_subdomain"] == True]["sld"].value_counts(
        normalize=True).rename_axis('domain').reset_index(name='count')
print(common_bad_domains[:5])
common_bad_domains.to_csv(os.path.join(OUTPUT_PATH, "common_bad_domains.csv"),
                          index=False)

bad_domains["legit"] = 0

print("loading legiturls")
legit_domains = pd.read_csv("known.txt",
                            header=None,
                            names=["domain"])

legit_domains["legit"] = 1

urls = pd.concat([legit_domains, bad_domains
                  ])[["domain",
                      "legit"]].drop_duplicates(subset="domain").sample(frac=1)
print(urls[:5])


urls.to_csv(os.path.join(OUTPUT_PATH, "dataset.csv"), index=False)