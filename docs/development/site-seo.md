# Website SEO and indexing

The static website is in `site/`. The Pages workflow publishes that directory
after changes reach `main`. Its current public base URL is
`https://sapphir3-ros3.github.io/Solomon/`.

Each indexable page has a unique title, description, absolute canonical URL,
social metadata, one descriptive H1 and content present in the initial HTML.
`site/sitemap.xml` lists the hand-authored canonical page URLs, without section
fragments. The Pages build automatically adds the canonical URLs of generated
Wiki pages to the published sitemap, keeping it synchronized with `docs/`.
Add new hand-authored public pages to the source sitemap and link to them from
existing pages.

Structured data describes the software on the home page and the breadcrumb on
the quickstart. It does not promise Google rich results: no reviews, ratings or
commercial offers are invented.

## After publishing

1. Add a URL-prefix property for `https://sapphir3-ros3.github.io/Solomon/` in
   [Google Search Console](https://search.google.com/search-console).
2. Verify ownership using the method offered for your account. If using an HTML
   verification file, put Google's exact file in `site/` and publish it; if using
   a verification meta tag, add Google's exact tag to the home page. Preserve the
   verification file or tag after verification.
3. Submit `https://sapphir3-ros3.github.io/Solomon/sitemap.xml` in the Sitemaps report.
4. Inspect both page URLs. Check the live test, indexing status and selected
   canonical; request indexing after significant changes.
5. Monitor impressions, queries, clicks and indexing errors. A search using
   `site:` or a third-party search tool is not definitive evidence of indexing.

Google reads `robots.txt` at the origin root,
`https://sapphir3-ros3.github.io/robots.txt`, not at `/Solomon/robots.txt`.
This project deployment cannot manage the origin root. At the time of the audit,
the root robots file returned 404; an absent robots file does not block crawling.
Do not add a project-path robots file and assume it controls Googlebot or
discovers the sitemap. Submit the sitemap through Search Console instead.

The generic name Solomon competes with unrelated entities. Use a clear product
description such as "Solomon AI agent" in relevant public references and
link to the official site from the repository. A dedicated domain is an optional
future improvement; Google's site-name feature does not support subdirectory
sites. If the domain changes, update all canonical URLs, sitemap URLs and
absolute structured-data and social URLs together, and arrange redirects.

SEO changes only take effect publicly after deployment and recrawling. Sitemap
submission helps discovery but does not guarantee indexing or rankings.

References: [Google sitemap guidance](https://developers.google.com/search/docs/crawling-indexing/sitemaps/build-sitemap),
[robots.txt scope](https://developers.google.com/search/docs/crawling-indexing/robots/intro),
[site names](https://developers.google.com/search/docs/appearance/site-names).
