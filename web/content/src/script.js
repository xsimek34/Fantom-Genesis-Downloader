let reloadInterval = 10000000;

function reload() {
    setTimeout(function () {
        updateData();
        reload();
    }, reloadInterval);
}

$.getJSON('/api/data', function (data) {
    updateTables(data);
});

function updateData() {
    $.getJSON('/api/data', function (data) {
        updateTables(data);
    });
}

function updateTables(data) {
    data.categories.forEach(category => {
        let category_label = document.getElementById(category.name + '_label');
        if (category_label == null) {
            let label = document.createElement('h2');
            label.id = category.name + '_label';
            label.innerHTML = category.description;

            document.getElementById('content').appendChild(label);
        }

        let category_head = document.getElementById(category.name + '_head');
        if (category_head == null) {
            let table = document.createElement('table');
            table.classList.add('files');
            table.id = category.name + '_table';

            table.appendChild(getTableHead(category.name));
            document.getElementById('content').appendChild(table);
        }

        let category_body = document.getElementById(category.name + '_body');
        if (category_body == null) {
            document.getElementById(category.name + '_table').appendChild(getTableContent(data, category.name));
        } else {
            category_body.parentNode.replaceChild(getTableContent(data, category.name), category_body);
        }
    });
}

function getTableHead(name) {
    let thead = document.createElement('thead');
    thead.id = name + '_head';

    let row_0 = thead.insertRow();

    let download_links = row_0.appendChild(document.createElement('th'));
    download_links.colSpan = 2;
    download_links.innerHTML = 'Download Links';

    let starting_points = row_0.appendChild(document.createElement('th'));
    starting_points.colSpan = 2;
    starting_points.innerHTML = 'Starting Point';

    let options_included = row_0.appendChild(document.createElement('th'));
    options_included.colSpan = 4;
    options_included.innerHTML = 'Options Included';

    let file_size = row_0.appendChild(document.createElement('th'));
    file_size.rowSpan = 2;
    file_size.innerHTML = 'File Size';

    let row_1 = thead.insertRow();

    let genesis_file = row_1.appendChild(document.createElement('th'));
    genesis_file.innerHTML = 'Genesis File';

    let checksum = row_1.appendChild(document.createElement('th'));
    checksum.innerHTML = 'Checksum';

    let epoch = row_1.appendChild(document.createElement('th'));
    epoch.innerHTML = 'Epoch';

    let block = row_1.appendChild(document.createElement('th'));
    block.innerHTML = 'Block';

    let fullsync = row_1.appendChild(document.createElement('th'));
    fullsync.innerHTML = 'Fullsync';

    let snapsync = row_1.appendChild(document.createElement('th'));
    snapsync.innerHTML = 'Snapsync';

    let blocks_history = row_1.appendChild(document.createElement('th'));
    blocks_history.innerHTML = 'Blocks History';

    let starting_evm_history = row_1.appendChild(document.createElement('th'));
    starting_evm_history.innerHTML = 'Starting EVM History';

    return thead;
}

function getTableContent(data, category_name) {
    let tbody = document.createElement('tbody');
    tbody.id = category_name + '_body';

    data.genesis_files.forEach(genesis_file => {
        if (genesis_file.category == category_name) {
            let row = tbody.insertRow();

            let file_url = document.createElement('a');
            file_url.innerHTML = genesis_file.name;
            if (genesis_file.static) {
                file_url.href = '/static/' + genesis_file.name;
            } else {
                file_url.href = '/dynamic/' + genesis_file.name;
            }

            let name = row.insertCell();
            name.appendChild(file_url);

            let md5_url = document.createElement('a');
            md5_url.innerHTML = 'MD5';
            md5_url.href = '/md5/' + genesis_file.md5;

            let md5 = row.insertCell();
            if (genesis_file.md5 == '') {
                md5.innerHTML = 'MD5';
            } else {
                md5.appendChild(md5_url);
            }

            let epoch = row.insertCell();
            epoch.classList.add('number');
            epoch.innerHTML = genesis_file.epoch.toLocaleString('en-US');

            let block = row.insertCell();
            block.classList.add('number');
            block.innerHTML = genesis_file.block.toLocaleString('en-US');

            let fullsync = row.insertCell();
            fullsync.classList.add('flag');
            if (genesis_file.fullsync) {
                fullsync.innerHTML = 'Yes';
            } else {
                fullsync.innerHTML = 'No';
            }

            let snapsync = row.insertCell();
            snapsync.classList.add('flag');
            if (genesis_file.snapsync) {
                snapsync.innerHTML = 'Yes';
            } else {
                snapsync.innerHTML = 'No';
            }

            let blocks_history = row.insertCell();
            blocks_history.classList.add('flag');
            blocks_history.innerHTML = genesis_file.block_history;

            let evm_history = row.insertCell();
            evm_history.classList.add('flag');
            evm_history.innerHTML = genesis_file.evm_history;

            let file_size = row.insertCell();
            file_size.classList.add('number');
            file_size.innerHTML = formatBytes(genesis_file.file_size);
        }
    });
    return tbody;
}

function formatBytes(bytes, decimals = 2) {
    if (!+bytes) return '0 Bytes'

    const k = 1024
    const dm = decimals < 0 ? 0 : decimals
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB']

    const i = Math.floor(Math.log(bytes) / Math.log(k))

    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`
}