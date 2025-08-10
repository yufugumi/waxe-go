# Wellington Axe Runners (WAXE)

This is a series of Typescript scripts run as cron jobs via GitHub Actions. Set to run at the start of every month, Playwright and axe-core will test [wellington.govt.nz](https://wellington.govt.nz), [letstalk.wellington.govt.nz](https://letstalk.wellington.govt.nz), and [archivesonline.wcc.govt.nz](https://archivesonline.wcc.govt.nz). It records these errors in a CSV for each website, which is then added to a release on GitHub.

> [!NOTE]
> This project is still very much a work in progress and has some rough edges. While it could technically be used to mass test other websites with [axe-core](https://github.com/dequelabs/axe-core), it would require further changes to make it work more flexible.

## Known issues

- the Playwright tests for services.wellington.govt.nz forms are rushed and very flakey. They need greater assertions.
- CSVs are a bit of a hacky way to store the data. They're not very searchable or filterable, and multiple areas of the same issue are stored in a single cell which makes it hard to read. Most sites use HTML reports now but this needs further refinement.
- there isn't a lot of performance testing, and the tests are slow to run.
